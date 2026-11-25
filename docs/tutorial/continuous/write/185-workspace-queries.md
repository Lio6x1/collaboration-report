# 分步编写 `services/document/internal/repository/document/workspace_queries.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/185-workspace-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供空间、知识库、文档目录与初始主线/支线的持久化操作。

创建一个可见文档需要完整结构、清单、正文、lineage和支线，不能只插document一行。 知识库名称搜索也在本服务私库内先加WHERE，再执行有界候选LIMIT；昵称则由Account提供，不能跨私库联表。

写之前先按这条执行路线理解各段的连接：

1. CreateInitialStructure/Style/Manifest建立不可变依赖
2. InsertInitialContent和InsertInitialLineage写真实来源
3. ActivateDocument最后公开
4. DocumentCollaboratorCandidates用UNION去重授权对象。
5. KnowledgeBaseCandidates消费可选search首项，用space/lifecycle/id>last和字面LOCATE(LOWER(name))过滤，再按id升序取budget条。
6. KBMemberCandidates只给本库ACTIVE角色候选与revision；服务层解析Account姓名、按q筛选及重验资格后才组成可见成员页。

本文件需要守住的边界：

- 账号姓名不在Document私库联表
- 协作者候选不是授权
- 首次空间物化需要持久gate防重复。
- budget是授权扫描预算，不是可见总数；不能把候选条数当用户有权看到的成员人数。
- search ...string只消费首项，缺省保持原目录；参数值必须走?占位符，不能把q拼成SQL条件或LIKE通配符。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/workspace_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/workspace_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/workspace_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/workspace_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:323 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

// directory.go：保留原实现，按业务合并文件。

````

## 第 2 步：ExplicitKBVisibility

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func ExplicitKBVisibility(ctx context.Context, q Queryer, kb, actor uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;) OR EXISTS(SELECT 1 FROM knowledge_base_creatio …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;) OR EXISTS(SELECT 1 FROM knowledge_base_creatio …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;) OR EXISTS(SELECT 1 FROM knowledge_base_creatio …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>visible</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;) OR EXISTS(SELECT 1 FROM knowledge_base_creatio …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;) OR EXISTS(SELECT 1 FROM knowledge_base_creatio …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return visible, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=323:751 -->
````go
func ExplicitKBVisibility(ctx context.Context, q Queryer, kb, actor uint64) (bool, error) {
	var visible bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE') OR EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE')", kb, actor, kb, actor).Scan(&visible)
	return visible, err
}
````

## 第 3 步：DerivedDirectorySources

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func DerivedDirectorySources(ctx context.Context, q Queryer, kb, actor uint64) ([]domain.DirectorySource, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT d.id,d.acl_revision FROM document d WHERE d.knowledge_base_id=? AND d.lifecycle=&#x27;ACTIVE&#x27; AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM content_gr …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT d.id,d.acl_revision FROM document d WHERE d.knowledge_base_id=? AND d.lifecycle=&#x27;ACTIVE&#x27; AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM content_gr …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT d.id,d.acl_revision FROM document d WHERE d.knowledge_base_id=? AND d.lifecycle=&#x27;ACTIVE&#x27; AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM content_gr …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.DirectorySource</code> | 使用 <code>domain.DirectorySource</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DIRECTORY_SOURCE_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT d.id,d.acl_revision FROM document d WHERE d.knowledge_base_id=? AND d.lifecycle=&#x27;ACTIVE&#x27; AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM content_gr …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;source.ID, &amp;source.Epoch) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;DIRECTORY_SOURCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;source.ID, &amp;source.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return sources, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=751:1859 -->
````go
func DerivedDirectorySources(ctx context.Context, q Queryer, kb, actor uint64) ([]domain.DirectorySource, error) {
	rows, err := q.QueryContext(ctx, "SELECT d.id,d.acl_revision FROM document d WHERE d.knowledge_base_id=? AND d.lifecycle='ACTIVE' AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM content_grant g WHERE g.document_id=d.id AND g.user_id=? AND g.state='ACTIVE' AND (g.capability<>'EDIT' OR EXISTS(SELECT 1 FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=d.current_structure_id WHERE u.document_id=d.id AND u.retired_structure_id IS NULL AND ((g.scope_kind='UNIT' AND g.scope_id=u.id) OR (g.scope_kind='CHAPTER' AND g.scope_id=u.chapter_id)))))) ORDER BY d.id LIMIT 401", kb, actor, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []domain.DirectorySource
	for rows.Next() {
		var source domain.DirectorySource
		if rows.Scan(&source.ID, &source.Epoch) != nil {
			return nil, errors.New("DIRECTORY_SOURCE_UNAVAILABLE")
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}
````

## 第 4 步：KBCreationAllowed

插入位置：第 3 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func KBCreationAllowed(ctx context.Context, tx *sql.Tx, kb, actor uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, kb, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, kb, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, kb, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>allowed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, kb, actor).Scan(&amp;allowed)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, kb, actor)</code>。
- 此处通过 <code>return allowed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1859:2169 -->
````go
func KBCreationAllowed(ctx context.Context, tx *sql.Tx, kb, actor uint64) (bool, error) {
	var allowed bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE')", kb, actor).Scan(&allowed)
	return allowed, err
}
````

## 第 5 步：KnowledgeBaseCandidates

插入位置：第 4 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func KnowledgeBaseCandidates(ctx context.Context, db *sql.DB, space uint64, last string, budget int, search ...string) ([]uint64, error)`。

**这一段为什么这样写：** 将知识库名称条件放在SQL候选扫描之前，使后页名称能被直接检索，并保留当前授权层继续裁剪。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次有界目录查询上下文。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle=&#x27;ACTIVE&#x27; AND id&gt;? AND (?=&#x27;&#x27; OR LOCATE(?,LOWER(name))&gt;0) ORDER BY id LIMIT ?&quot;, space, last, query, query, budget)</code> |
| <code>db</code> | <code>*sql.DB</code> | Document自己的SQL连接池，不访问Account私库。<br>在此声明中的实际使用：<code>db.QueryContext</code> |
| <code>space</code> | <code>uint64</code> | 当前准入空间ID。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle=&#x27;ACTIVE&#x27; AND id&gt;? AND (?=&#x27;&#x27; OR LOCATE(?,LOWER(name))&gt;0) ORDER BY id LIMIT ?&quot;, space, last, query, query, budget)</code> |
| <code>last</code> | <code>string</code> | 签名游标解析出的上一知识库ID，首页为0。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle=&#x27;ACTIVE&#x27; AND id&gt;? AND (?=&#x27;&#x27; OR LOCATE(?,LOWER(name))&gt;0) ORDER BY id LIMIT ?&quot;, space, last, query, query, budget)</code> |
| <code>budget</code> | <code>int</code> | 服务层有界扫描上限limit*10+1；可见页仍只输出limit项。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle=&#x27;ACTIVE&#x27; AND id&gt;? AND (?=&#x27;&#x27; OR LOCATE(?,LOWER(name))&gt;0) ORDER BY id LIMIT ?&quot;, space, last, query, query, budget)</code> |
| <code>search</code> | <code>...string</code> | 可选已trim/lower的单个名称q；空或未传不加文字筛选。<br>在此声明中的实际使用：<code>len(search)</code>；<code>query = search&#91;0&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 名称匹配的知识库ID候选，仍需逐项核验当前可见来源。<br>实际返回表达式：<code>nil</code>；<code>candidates</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | SQL/扫描故障。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(search) &gt; 0</code>。
- 执行 <code>db.QueryContext(ctx, &quot;SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle=&#x27;ACTIVE&#x27; AND id&gt;? AND (?=&#x27;&#x27; OR LOCATE(?,LOWER(name))&gt;0) ORDER BY id LIMIT ?&quot;, space, last, query, query, budget)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;id) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return candidates, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2169:2889 -->
````go
func KnowledgeBaseCandidates(ctx context.Context, db *sql.DB, space uint64, last string, budget int, search ...string) ([]uint64, error) {
	query := ""
	if len(search) > 0 {
		query = search[0]
	}
	rows, err := db.QueryContext(ctx, "SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle='ACTIVE' AND id>? AND (?='' OR LOCATE(?,LOWER(name))>0) ORDER BY id LIMIT ?", space, last, query, query, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
		}
		candidates = append(candidates, id)
	}
	return candidates, rows.Err()
}
````

## 第 6 步：DocumentCandidates

插入位置：第 5 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func DocumentCandidates(ctx context.Context, db *sql.DB, space, kb uint64, last string, budget int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, statement, args...)</code> |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{space, last}</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>kb != 0</code>；<code>append(args, kb)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{space, last}</code> |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>append(args, budget)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>candidates</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>kb != 0</code>。
- 执行 <code>db.QueryContext(ctx, statement, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;id) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return candidates, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2889:3645 -->
````go
func DocumentCandidates(ctx context.Context, db *sql.DB, space, kb uint64, last string, budget int) ([]uint64, error) {
	statement := "SELECT id FROM document WHERE space_id=? AND lifecycle='ACTIVE' AND id>?"
	args := []any{space, last}
	if kb != 0 {
		statement += " AND knowledge_base_id=?"
		args = append(args, kb)
	}
	statement += " ORDER BY id LIMIT ?"
	args = append(args, budget)
	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
		}
		candidates = append(candidates, id)
	}
	return candidates, rows.Err()
}
````

## 第 7 步：KBMemberCandidates

插入位置：第 6 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func KBMemberCandidates(ctx context.Context, db *sql.DB, kb uint64, last string, budget int) ([]dto.Member, error)`。

**这一段为什么这样写：** 只读取本库当前ACTIVE成员候选，姓名与用户身份在服务层向Account解析，不在Document数据库复制一套账号权威。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 请求取消上下文。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state=&#x27;ACTIVE&#x27; AND user_id&gt;? ORDER BY user_id LIMIT ?&quot;, kb, last, budget)</code> |
| <code>db</code> | <code>*sql.DB</code> | Document私库连接池。<br>在此声明中的实际使用：<code>db.QueryContext</code> |
| <code>kb</code> | <code>uint64</code> | 已通过Manage核验的知识库ID。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state=&#x27;ACTIVE&#x27; AND user_id&gt;? ORDER BY user_id LIMIT ?&quot;, kb, last, budget)</code> |
| <code>last</code> | <code>string</code> | 签名游标的上一账号ID边界。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state=&#x27;ACTIVE&#x27; AND user_id&gt;? ORDER BY user_id LIMIT ?&quot;, kb, last, budget)</code> |
| <code>budget</code> | <code>int</code> | 扫描预算上限；不能用它宣称可见成员总数。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state=&#x27;ACTIVE&#x27; AND user_id&gt;? ORDER BY user_id LIMIT ?&quot;, kb, last, budget)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.Member</code> | 带user_id/role/state/revision的候选，DisplayName尚待权威解析。<br>实际返回表达式：<code>nil</code>；<code>candidates</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | SQL或扫描失败。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;KB_MEMBERS_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state=&#x27;ACTIVE&#x27; AND user_id&gt;? ORDER BY user_id LIMIT ?&quot;, kb, last, budget)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;member.UserId, &amp;member.Role, &amp;member.State, &amp;member.Revision) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;KB_MEMBERS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;member.UserId, &amp;member.Role, &amp;member.State, &amp;member.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return candidates, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3645:4380 -->
````go
func KBMemberCandidates(ctx context.Context, db *sql.DB, kb uint64, last string, budget int) ([]dto.Member, error) {
	rows, err := db.QueryContext(ctx, "SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state='ACTIVE' AND user_id>? ORDER BY user_id LIMIT ?", kb, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []dto.Member
	for rows.Next() {
		var member dto.Member
		if rows.Scan(&member.UserId, &member.Role, &member.State, &member.Revision) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "KB_MEMBERS_UNAVAILABLE")
		}
		candidates = append(candidates, member)
	}
	return candidates, rows.Err()
}
````

## 第 8 步：LockedKBMember

插入位置：第 7 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func LockedKBMember(ctx context.Context, tx *sql.Tx, kb uint64, user string) (string, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, user).Scan(&amp;revision, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, user)</code>。
- 此处通过 <code>return revision, state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4380:4726 -->
````go
func LockedKBMember(ctx context.Context, tx *sql.Tx, kb uint64, user string) (string, string, error) {
	var revision, state string
	err := tx.QueryRowContext(ctx, "SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE", kb, user).Scan(&revision, &state)
	return revision, state, err
}

````

## 第 9 步：DocumentCollaboratorCandidates

插入位置：第 8 步代码之后，紧接追加；完成稿约第 106 行。

文档协作者来自本服务的现行授权事实；UNION同时去掉多个授权范围和负责人产生的重复项。
这里只读取Document私有库，不把账号资料表联进SQL，也不把候选当成新的访问授权。

本步声明：`func DocumentCollaboratorCandidates(ctx context.Context, q Queryer, document uint64, last string, budget int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT candidate.user_id FROM ( 		SELECT user_id FROM content_grant WHERE document_id=? AND state=&#x27;ACTIVE&#x27; 		UNION SELECT responsible_user_id FROM document WHE …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT candidate.user_id FROM ( 		SELECT user_id FROM content_grant WHERE document_id=? AND state=&#x27;ACTIVE&#x27; 		UNION SELECT responsible_user_id FROM document WHE …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT candidate.user_id FROM ( 		SELECT user_id FROM content_grant WHERE document_id=? AND state=&#x27;ACTIVE&#x27; 		UNION SELECT responsible_user_id FROM document WHE …</code>（长表达式，按本步完整代码继续） |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT candidate.user_id FROM ( 		SELECT user_id FROM content_grant WHERE document_id=? AND state=&#x27;ACTIVE&#x27; 		UNION SELECT responsible_user_id FROM document WHE …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>candidates</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT candidate.user_id FROM ( 		SELECT user_id FROM content_grant WHERE document_id=? AND state=&#x27;ACTIVE&#x27; 		UNION SELECT responsible_user_id FROM document WHE …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;user)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return candidates, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4726:5939 -->
````go
// 文档协作者来自本服务的现行授权事实；UNION同时去掉多个授权范围和负责人产生的重复项。
// 这里只读取Document私有库，不把账号资料表联进SQL，也不把候选当成新的访问授权。
func DocumentCollaboratorCandidates(ctx context.Context, q Queryer, document uint64, last string, budget int) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT candidate.user_id FROM (
		SELECT user_id FROM content_grant WHERE document_id=? AND state='ACTIVE'
		UNION SELECT responsible_user_id FROM document WHERE id=? AND responsible_user_id IS NOT NULL
		UNION SELECT s.account_owner_id FROM document d JOIN space s ON s.id=d.space_id
			WHERE d.id=? AND s.kind='PERSONAL' AND s.account_owner_id IS NOT NULL
	) candidate WHERE candidate.user_id>? ORDER BY candidate.user_id LIMIT ?`, document, document, document, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []uint64
	for rows.Next() {
		var user uint64
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		candidates = append(candidates, user)
	}
	return candidates, rows.Err()
}

// initialization.go：保留原实现，按业务合并文件。

````

## 第 10 步：InsertID

插入位置：第 9 步代码之后，紧接追加；完成稿约第 132 行。

本步声明：`func InsertID(ctx context.Context, tx *sql.Tx, statement string, args ...any) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>statement</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, args...)</code> |
| <code>args</code> | <code>...any</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, args...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, statement, args...)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5939:6187 -->
````go
func InsertID(ctx context.Context, tx *sql.Tx, statement string, args ...any) (uint64, error) {
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}
````

## 第 11 步：CreateDocument

插入位置：第 10 步代码之后，紧接追加；完成稿约第 140 行。

本步声明：`func CreateDocument(ctx context.Context, tx *sql.Tx, space, kb, actor uint64, title, visibility string, locked bool) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>kb != 0</code>；<code>knowledgeBase = kb</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |
| <code>title</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |
| <code>visibility</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |
| <code>locked</code> | <code>bool</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INITIALIZI …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>kb != 0</code>。
- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,&#x27;INI …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6187:6652 -->
````go
func CreateDocument(ctx context.Context, tx *sql.Tx, space, kb, actor uint64, title, visibility string, locked bool) (uint64, error) {
	var knowledgeBase any
	if kb != 0 {
		knowledgeBase = kb
	}
	return InsertID(ctx, tx, "INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,'INITIALIZING',?,'UNPUBLISHED')", space, knowledgeBase, actor, title, visibility, locked)
}
````

## 第 12 步：CreateInitialStructure

插入位置：第 11 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`func CreateInitialStructure(ctx context.Context, tx *sql.Tx, document uint64, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,&#x27;structure-v1&#x27;)&quot;, document, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6652:6924 -->
````go
func CreateInitialStructure(ctx context.Context, tx *sql.Tx, document uint64, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,'structure-v1')", document, operation)
}
````

## 第 13 步：CreateInitialStyle

插入位置：第 12 步代码之后，紧接追加；完成稿约第 150 行。

本步声明：`func CreateInitialStyle(ctx context.Context, tx *sql.Tx, document uint64, style, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> |
| <code>style</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,&#x27;style-v1&#x27;,?)&quot;, document, style, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6924:7211 -->
````go
func CreateInitialStyle(ctx context.Context, tx *sql.Tx, document uint64, style, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,'style-v1',?)", document, style, operation)
}
````

## 第 14 步：CreateInitialManifest

插入位置：第 13 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func CreateInitialManifest(ctx context.Context, tx *sql.Tx, document, structure, style uint64, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code>；<code>&quot;INITIAL:&quot;+operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, structure, style, operation, &quot;INITIAL:&quot;+operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,&#x27;INITIAL&#x27;,?)&quot;, document, stru …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7211:7572 -->
````go
func CreateInitialManifest(ctx context.Context, tx *sql.Tx, document, structure, style uint64, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,'INITIAL',?)", document, structure, style, operation, "INITIAL:"+operation)
}
````

## 第 15 步：CreateInitialChapter

插入位置：第 14 步代码之后，紧接追加；完成稿约第 156 行。

本步声明：`func CreateInitialChapter(ctx context.Context, tx *sql.Tx, document, structure uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)&quot;, document, structure)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7572:7795 -->
````go
func CreateInitialChapter(ctx context.Context, tx *sql.Tx, document, structure uint64) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)", document, structure)
}
````

## 第 16 步：CreateInitialUnit

插入位置：第 15 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func CreateInitialUnit(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> |
| <code>chapter</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,&#x27;CHAPTER&#x27;,?)&quot;, document, chapter, structure)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7795:8072 -->
````go
func CreateInitialUnit(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,'CHAPTER',?)", document, chapter, structure)
}
````

## 第 17 步：InsertInitialStructureEntry

插入位置：第 16 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func InsertInitialStructureEntry(ctx context.Context, tx *sql.Tx, document, structure, chapter, unit uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,&#x27;CHAPTER&#x27;,&#x27;第一章&#x27;,1,?)&quot;, structure, document, chapter, chapter, unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,&#x27;CHAPTER&#x27;,&#x27;第一章&#x27;,1,?)&quot;, structure, document, chapter, chapter, unit)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,&#x27;CHAPTER&#x27;,&#x27;第一章&#x27;,1,?)&quot;, structure, document, chapter, chapter, unit)</code> |
| <code>chapter</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,&#x27;CHAPTER&#x27;,&#x27;第一章&#x27;,1,?)&quot;, structure, document, chapter, chapter, unit)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,&#x27;CHAPTER&#x27;,&#x27;第一章&#x27;,1,?)&quot;, structure, document, chapter, chapter, unit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,&#x27;CHAPTER&#x27;,&#x27;第一章&#x27;,1,?)&quot;, structure, document, chapter, chapter, unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=8072:8427 -->
````go
func InsertInitialStructureEntry(ctx context.Context, tx *sql.Tx, document, structure, chapter, unit uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,'CHAPTER','第一章',1,?)", structure, document, chapter, chapter, unit)
	return err
}
````

## 第 18 步：InsertInitialManifestEntry

插入位置：第 17 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func InsertInitialManifestEntry(ctx context.Context, tx *sql.Tx, document, manifest, unit, version uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)&quot;, manifest, document, unit, version)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)&quot;, manifest, document, unit, version)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)&quot;, manifest, document, unit, version)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)&quot;, manifest, document, unit, version)</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)&quot;, manifest, document, unit, version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)&quot;, manifest, document, unit, version)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8427:8720 -->
````go
func InsertInitialManifestEntry(ctx context.Context, tx *sql.Tx, document, manifest, unit, version uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", manifest, document, unit, version)
	return err
}
````

## 第 19 步：ActivateDocument

插入位置：第 18 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func ActivateDocument(ctx context.Context, tx *sql.Tx, document, structure, style, manifest uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND lifecycle=&#x27;INITIALIZING&#x27;&quot;, structure, style, manifest, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND lifecycle=&#x27;INITIALIZING&#x27;&quot;, structure, style, manifest, document)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND lifecycle=&#x27;INITIALIZING&#x27;&quot;, structure, style, manifest, document)</code> |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND lifecycle=&#x27;INITIALIZING&#x27;&quot;, structure, style, manifest, document)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND lifecycle=&#x27;INITIALIZING&#x27;&quot;, structure, style, manifest, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND lifecycle=&#x27;INITIALIZING&#x27;&quot;, structure, style, manifest, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=8720:9057 -->
````go
func ActivateDocument(ctx context.Context, tx *sql.Tx, document, structure, style, manifest uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle='ACTIVE' WHERE id=? AND lifecycle='INITIALIZING'", structure, style, manifest, document)
	return err
}
````

## 第 20 步：InsertInitialContent

插入位置：第 19 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func InsertInitialContent(ctx context.Context, tx *sql.Tx, document, unit, structure uint64, operation string, content domain.InitialContent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_ope …</code>（长表达式，按本步完整代码继续）；<code>ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_ope …</code>（长表达式，按本步完整代码继续）；<code>ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_ope …</code>（长表达式，按本步完整代码继续）；<code>domain.TextID(unit)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_ope …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_ope …</code>（长表达式，按本步完整代码继续）；<code>ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)</code> |
| <code>content</code> | <code>domain.InitialContent</code> | 使用 <code>domain.InitialContent</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>在此声明中的实际使用：<code>content.Version</code>；<code>content.Canonical</code>；<code>content.Hash</code>；<code>content.Provenance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_ope …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=9057:9868 -->
````go
func InsertInitialContent(ctx context.Context, tx *sql.Tx, document, unit, structure uint64, operation string, content domain.InitialContent) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,'content-v1','canonical-v1',?,?,'INITIAL',?,?,'provenance-v1')", content.Version, document, unit, structure, string(content.Canonical), content.Canonical, content.Hash[:], len(content.Canonical), operation, domain.TextID(unit), string(content.Provenance))
	if err != nil {
		return err
	}
	return ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)
}
````

## 第 21 步：CreateInitialBranch

插入位置：第 20 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`func CreateInitialBranch(ctx context.Context, tx *sql.Tx, document, unit, version uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,&#x27;PREPARING&#x27;,?)&quot;, document, unit, version)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=9868:10132 -->
````go
func CreateInitialBranch(ctx context.Context, tx *sql.Tx, document, unit, version uint64) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,'PREPARING',?)", document, unit, version)
}
````

## 第 22 步：InsertInitialLineage

插入位置：第 21 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func InsertInitialLineage(ctx context.Context, tx *sql.Tx, document, unit, branch, manifest uint64, operation string, content domain.InitialContent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续） |
| <code>content</code> | <code>domain.InitialContent</code> | 使用 <code>domain.InitialContent</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>在此声明中的实际使用：<code>content.Lineage</code>；<code>content.Version</code>；<code>content.Provenance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema)  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=10132:10653 -->
````go
func InsertInitialLineage(ctx context.Context, tx *sql.Tx, document, unit, branch, manifest uint64, operation string, content domain.InitialContent) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema) VALUES(?,?,?,?,'CREATED',?,?,?,?,'provenance-v1')", content.Lineage, branch, unit, document, operation, manifest, content.Version, string(content.Provenance))
	return err
}
````

## 第 23 步：ActivateInitialBranch

插入位置：第 22 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`func ActivateInitialBranch(ctx context.Context, tx *sql.Tx, branch, lineage uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=?&quot;, lineage, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=?&quot;, lineage, branch)</code> |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=?&quot;, lineage, branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;ACTIVE&#x27; WHERE id=?&quot;, lineage, branch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=10653:10879 -->
````go
func ActivateInitialBranch(ctx context.Context, tx *sql.Tx, branch, lineage uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,lifecycle='ACTIVE' WHERE id=?", lineage, branch)
	return err
}
````

## 第 24 步：BindInitialBranch

插入位置：第 23 步代码之后，紧接追加；完成稿约第 192 行。

本步声明：`func BindInitialBranch(ctx context.Context, tx *sql.Tx, document, unit, branch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?&quot;, branch, unit, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?&quot;, branch, unit, document)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?&quot;, branch, unit, document)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?&quot;, branch, unit, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?&quot;, branch, unit, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=10879:11124 -->
````go
func BindInitialBranch(ctx context.Context, tx *sql.Tx, document, unit, branch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?", branch, unit, document)
	return err
}
````

## 第 25 步：LockedCreationKnowledgeBase

插入位置：第 24 步代码之后，紧接追加；完成稿约第 196 行。

本步声明：`func LockedCreationKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64) (domain.CreationKnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>CreationKnowledgeBase(ctx, tx, id, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>CreationKnowledgeBase(ctx, tx, id, true)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>CreationKnowledgeBase(ctx, tx, id, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.CreationKnowledgeBase</code> | 使用 <code>domain.CreationKnowledgeBase</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>由 <code>CreationKnowledgeBase(ctx, tx, id, true)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>CreationKnowledgeBase(ctx, tx, id, true)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return CreationKnowledgeBase(ctx, tx, id, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=11124:11293 -->
````go
func LockedCreationKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64) (domain.CreationKnowledgeBase, error) {
	return CreationKnowledgeBase(ctx, tx, id, true)
}

````

## 第 26 步：CreationKnowledgeBase

插入位置：第 25 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func CreationKnowledgeBase(ctx context.Context, q Queryer, id uint64, lock bool) (domain.CreationKnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.CreationKnowledgeBase</code> | 使用 <code>domain.CreationKnowledgeBase</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>实际返回表达式：<code>kb</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=?&quot;+suffix, id).Scan(&amp;kb.Space, &amp;kb.Lifecycle, &amp;kb.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=?&quot;+suffix, id)</code>。
- 此处通过 <code>return kb, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=11293:11683 -->
````go
func CreationKnowledgeBase(ctx context.Context, q Queryer, id uint64, lock bool) (domain.CreationKnowledgeBase, error) {
	var kb domain.CreationKnowledgeBase
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=?"+suffix, id).Scan(&kb.Space, &kb.Lifecycle, &kb.Revision)
	return kb, err
}

````

## 第 27 步：KnowledgeBaseACL

插入位置：第 26 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`func KnowledgeBaseACL(ctx context.Context, q Queryer, id uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=?&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=?&quot;, id).Scan(&amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=?&quot;, id)</code>。
- 此处通过 <code>return revision, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=11683:11916 -->
````go
func KnowledgeBaseACL(ctx context.Context, q Queryer, id uint64) (uint64, error) {
	var revision uint64
	err := q.QueryRowContext(ctx, "SELECT acl_revision FROM knowledge_base WHERE id=?", id).Scan(&revision)
	return revision, err
}
````

## 第 28 步：KnowledgeBaseCreationState

插入位置：第 27 步代码之后，紧接追加；完成稿约第 215 行。

本步声明：`func KnowledgeBaseCreationState(ctx context.Context, tx *sql.Tx, id, user uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?&quot;, id, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?&quot;, id, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?&quot;, id, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?&quot;, id, user).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?&quot;, id, user)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=11916:12269 -->
````go
func KnowledgeBaseCreationState(ctx context.Context, tx *sql.Tx, id, user uint64) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?", id, user).Scan(&state)
	return state, err
}

// knowledge_base.go：保留原实现，按业务合并文件。

````

## 第 29 步：KnowledgeBaseRow

插入位置：第 28 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`func KnowledgeBaseRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.KnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM knowledge_base WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM knowledge_base WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.KnowledgeBase</code> | 实际返回表达式：<code>kb</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM knowledge_ …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM knowledge_base WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>err != nil &#124;&#124; kb.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return kb, domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return kb, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=12269:12931 -->
````go
func KnowledgeBaseRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.KnowledgeBase, error) {
	var kb dto.KnowledgeBase
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM knowledge_base WHERE id=?"+suffix, id).Scan(&kb.Id, &kb.SpaceId, &kb.Name, &kb.Lifecycle, &kb.AclRevision, &kb.VisibilityPolicy, &kb.VisibilityRevision)
	if err != nil || kb.Lifecycle != "ACTIVE" {
		return kb, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
	}
	return kb, nil
}
````

## 第 30 步：KnowledgeBaseRole

插入位置：第 29 步代码之后，紧接追加；完成稿约第 235 行。

本步声明：`func KnowledgeBaseRole(ctx context.Context, q Queryer, id, user uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, user)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>role</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, user).Scan(&amp;role)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, user)</code>。
- 此处通过 <code>return role, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=12931:13212 -->
````go
func KnowledgeBaseRole(ctx context.Context, q Queryer, id, user uint64) (string, error) {
	var role string
	err := q.QueryRowContext(ctx, "SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE'", id, user).Scan(&role)
	return role, err
}
````

## 第 31 步：CreateKnowledgeBase

插入位置：第 30 步代码之后，紧接追加；完成稿约第 240 行。

本步声明：`func CreateKnowledgeBase(ctx context.Context, tx *sql.Tx, space uint64, name, visibility string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,&#x27;ACTIVE&#x27;,?)&quot;, space, name, visibility)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,&#x27;ACTIVE&#x27;,?)&quot;, space, name, visibility)</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,&#x27;ACTIVE&#x27;,?)&quot;, space, name, visibility)</code> |
| <code>visibility</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,&#x27;ACTIVE&#x27;,?)&quot;, space, name, visibility)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,&#x27;ACTIVE&#x27;,?)&quot;, space, name, visibility)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=13212:13580 -->
````go
func CreateKnowledgeBase(ctx context.Context, tx *sql.Tx, space uint64, name, visibility string) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,'ACTIVE',?)", space, name, visibility)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}
````

## 第 32 步：InsertInitialKBAdministrator

插入位置：第 31 步代码之后，紧接追加；完成稿约第 248 行。

本步声明：`func InsertInitialKBAdministrator(ctx context.Context, tx *sql.Tx, id, actor uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, actor)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=13580:13833 -->
````go
func InsertInitialKBAdministrator(ctx context.Context, tx *sql.Tx, id, actor uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", id, actor)
	return err
}
````

## 第 33 步：AvailableKBAdministrators

插入位置：第 32 步代码之后，紧接追加；完成稿约第 252 行。

本步声明：`func AvailableKBAdministrators(ctx context.Context, tx *sql.Tx, id uint64) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockAdministratorRetirementFacts(ctx, tx, 0, &amp;candidate)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockAdministratorRetirementFacts(ctx, tx, 0, &amp;candidate)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>KnowledgeBase: id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>candidate.Available</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return candidate.Available, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=13833:14286 -->
````go
func AvailableKBAdministrators(ctx context.Context, tx *sql.Tx, id uint64) (int, error) {
	// 调用方已持有KB锁。角色事务可能更早建立RR快照，必须用锁定当前读
	// 看到刚提交的注销预留，否则仍会把待退出者算作最后管理员的接任者。
	candidate := AdministratorRetirementCandidate{KnowledgeBase: id}
	err := lockAdministratorRetirementFacts(ctx, tx, 0, &candidate)
	return candidate.Available, err
}
````

## 第 34 步：KBAdministratorReservations

插入位置：第 33 步代码之后，紧接追加；完成稿约第 259 行。

本步声明：`func KBAdministratorReservations(ctx context.Context, tx *sql.Tx, id, user uint64) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>0</code>；<code>1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, id, user).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return 0, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>state == &quot;RESERVED&quot;</code>。条件成立时执行 <code>return 1, nil</code>。当前函数在这里结束。
- 此处通过 <code>return 0, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=14286:14690 -->
````go
func KBAdministratorReservations(ctx context.Context, tx *sql.Tx, id, user uint64) (int, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE", id, user).Scan(&state)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if state == "RESERVED" {
		return 1, nil
	}
	return 0, nil
}
````

## 第 35 步：SetKBRole

插入位置：第 34 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`func SetKBRole(ctx context.Context, tx *sql.Tx, id, user uint64, role, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1&quot;, id, user, role, state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1&quot;, id, user, role, state)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1&quot;, id, user, role, state)</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1&quot;, id, user, role, state)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1&quot;, id, user, role, state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1&quot;, id, user, role, state)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=14690:15024 -->
````go
func SetKBRole(ctx context.Context, tx *sql.Tx, id, user uint64, role, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1", id, user, role, state)
	return err
}
````

## 第 36 步：SetKBCreationGrant

插入位置：第 35 步代码之后，紧接追加；完成稿约第 277 行。

本步声明：`func SetKBCreationGrant(ctx context.Context, tx *sql.Tx, id, user uint64, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1&quot;, id, user, state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1&quot;, id, user, state)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1&quot;, id, user, state)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1&quot;, id, user, state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1&quot;, id, user, state)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=15024:15338 -->
````go
func SetKBCreationGrant(ctx context.Context, tx *sql.Tx, id, user uint64, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", id, user, state)
	return err
}
````

## 第 37 步：AdvanceKnowledgeBaseACL

插入位置：第 36 步代码之后，紧接追加；完成稿约第 281 行。

本步声明：`func AdvanceKnowledgeBaseACL(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=15338:15536 -->
````go
func AdvanceKnowledgeBaseACL(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?", id)
	return err
}
````

## 第 38 步：RenameKnowledgeBase

插入位置：第 37 步代码之后，紧接追加；完成稿约第 285 行。

本步声明：`func RenameKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64, name string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET name=?,acl_revision=acl_revision+1 WHERE id=?&quot;, name, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET name=?,acl_revision=acl_revision+1 WHERE id=?&quot;, name, id)</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET name=?,acl_revision=acl_revision+1 WHERE id=?&quot;, name, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET name=?,acl_revision=acl_revision+1 WHERE id=?&quot;, name, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=15536:15833 -->
````go
func RenameKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64, name string) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET name=?,acl_revision=acl_revision+1 WHERE id=?", name, id)
	return err
}

// knowledge_base_children.go：保留原实现，按业务合并文件。

````

## 第 39 步：KnowledgeBaseChildren

插入位置：第 38 步代码之后，紧接追加；完成稿约第 292 行。

本步声明：`func KnowledgeBaseChildren(ctx context.Context, q Queryer, id uint64, locked bool) ([]domain.KnowledgeBaseChild, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, statement, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, statement, id)</code> |
| <code>locked</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.KnowledgeBaseChild</code> | 使用 <code>domain.KnowledgeBaseChild</code> 类型；其定义处的职责：定义发布意图、正式指针水位及PUBLIC/SPACE_MEMBERS/COLLABORATORS受众判定。<br>实际返回表达式：<code>nil</code>；<code>children</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;KB_CHILDREN_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>locked</code>。
- 执行 <code>q.QueryContext(ctx, statement, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>locked</code>。
- 执行 <code>rows.Scan(&amp;child.ID, &amp;child.Epoch, &amp;release)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>rows.Scan(&amp;child.ID, &amp;child.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;KB_CHILDREN_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return children, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=15833:16820 -->
````go
func KnowledgeBaseChildren(ctx context.Context, q Queryer, id uint64, locked bool) ([]domain.KnowledgeBaseChild, error) {
	statement := "SELECT id,acl_revision FROM document WHERE knowledge_base_id=? AND lifecycle='ACTIVE' ORDER BY id LIMIT 401"
	if locked {
		statement = "SELECT id,acl_revision,current_release_id FROM document WHERE knowledge_base_id=? AND lifecycle='ACTIVE' ORDER BY id FOR UPDATE"
	}
	rows, err := q.QueryContext(ctx, statement, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var children []domain.KnowledgeBaseChild
	for rows.Next() {
		var child domain.KnowledgeBaseChild
		if locked {
			var release sql.NullInt64
			err = rows.Scan(&child.ID, &child.Epoch, &release)
			child.HasRelease = release.Valid
		} else {
			err = rows.Scan(&child.ID, &child.Epoch)
		}
		if err != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "KB_CHILDREN_UNAVAILABLE")
		}
		children = append(children, child)
	}
	return children, rows.Err()
}
````

## 第 40 步：CloseKnowledgeBaseForDeletion

插入位置：第 39 步代码之后，紧接追加；完成稿约第 319 行。

本步声明：`func CloseKnowledgeBaseForDeletion(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSING&#x27;,acl_revision=acl_revision+1 WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSING&#x27;,acl_revision=acl_revision+1 WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSING&#x27;,acl_revision=acl_revision+1 WHERE id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=16820:17044 -->
````go
func CloseKnowledgeBaseForDeletion(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle='CLOSING',acl_revision=acl_revision+1 WHERE id=?", id)
	return err
}
````

## 第 41 步：CloseKnowledgeBaseChildren

插入位置：第 40 步代码之后，紧接追加；完成稿约第 323 行。

本步声明：`func CloseKnowledgeBaseChildren(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE knowledge_base_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE knowledge_base_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE knowledge_base_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=17044:17397 -->
````go
func CloseKnowledgeBaseChildren(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='CLOSING',control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE knowledge_base_id=? AND lifecycle='ACTIVE'", id)
	return err
}

// management.go：保留原实现，按业务合并文件。

````

## 第 42 步：RenameDocument

插入位置：第 41 步代码之后，紧接追加；完成稿约第 330 行。

本步声明：`func RenameDocument(ctx context.Context, tx *sql.Tx, id uint64, title string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET title=?,control_revision=control_revision+1 WHERE id=?&quot;, title, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET title=?,control_revision=control_revision+1 WHERE id=?&quot;, title, id)</code> |
| <code>title</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET title=?,control_revision=control_revision+1 WHERE id=?&quot;, title, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET title=?,control_revision=control_revision+1 WHERE id=?&quot;, title, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=17397:17617 -->
````go
func RenameDocument(ctx context.Context, tx *sql.Tx, id uint64, title string) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET title=?,control_revision=control_revision+1 WHERE id=?", title, id)
	return err
}
````

## 第 43 步：TransferDocumentResponsibility

插入位置：第 42 步代码之后，紧接追加；完成稿约第 334 行。

本步声明：`func TransferDocumentResponsibility(ctx context.Context, tx *sql.Tx, id, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET responsible_user_id=?,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, user, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET responsible_user_id=?,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, user, id)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET responsible_user_id=?,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, user, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET responsible_user_id=?,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, user, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 43 bytes=17617:17886 -->
````go
func TransferDocumentResponsibility(ctx context.Context, tx *sql.Tx, id, user uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET responsible_user_id=?,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE id=?", user, id)
	return err
}
````

## 第 44 步：CloseDocument

插入位置：第 43 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`func CloseDocument(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=control_revision+1,acl_revision=acl_revision+1,publication_epoch=publication_epoch+1,publication_state …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=control_revision+1,acl_revision=acl_revision+1,publication_epoch=publication_epoch+1,publication_state …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=control_revision+1,acl_revision=acl_revision+1,publication_epoch=publication_epoch+1,publication_state …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=17886:18343 -->
````go
func CloseDocument(ctx context.Context, tx *sql.Tx, id uint64) error {
	// 历史release依赖保留在回收站；只撤下当前展示，不修改不可变正式内容。
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='CLOSING',control_revision=control_revision+1,acl_revision=acl_revision+1,publication_epoch=publication_epoch+1,publication_state='UNPUBLISHED',current_release_id=NULL WHERE id=? AND lifecycle='ACTIVE'", id)
	return err
}
````

## 第 45 步：NotifyResponsibilityTransfer

插入位置：第 44 步代码之后，紧接追加；完成稿约第 343 行。

本步声明：`func NotifyResponsibilityTransfer(ctx context.Context, tx *sql.Tx, user uint64, operation string, target []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,&#x27;RESPONSIBLE_TRANSFER&#x27;,?)&quot;, user, operation, string(target))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,&#x27;RESPONSIBLE_TRANSFER&#x27;,?)&quot;, user, operation, string(target))</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,&#x27;RESPONSIBLE_TRANSFER&#x27;,?)&quot;, user, operation, string(target))</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,&#x27;RESPONSIBLE_TRANSFER&#x27;,?)&quot;, user, operation, string(target))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 45 bytes=18343:18706 -->
````go
func NotifyResponsibilityTransfer(ctx context.Context, tx *sql.Tx, user uint64, operation string, target []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,'RESPONSIBLE_TRANSFER',?)", user, operation, string(target))
	return err
}

// space.go：保留原实现，按业务合并文件。

````

## 第 46 步：EnsureSpaceGate

插入位置：第 45 步代码之后，紧接追加；完成稿约第 350 行。

本步声明：`func EnsureSpaceGate(ctx context.Context, tx *sql.Tx, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 46 bytes=18706:18955 -->
````go
func EnsureSpaceGate(ctx context.Context, tx *sql.Tx, key string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", key)
	return err
}
````

## 第 47 步：LockSpaceGate

插入位置：第 46 步代码之后，紧接追加；完成稿约第 354 行。

本步声明：`func LockSpaceGate(ctx context.Context, tx *sql.Tx, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SPACE_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key).Scan(&amp;revision) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SPACE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key).Scan(&amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 47 bytes=18955:19229 -->
````go
func LockSpaceGate(ctx context.Context, tx *sql.Tx, key string) error {
	var revision uint64
	if tx.QueryRowContext(ctx, "SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&revision) != nil {
		return errors.New("SPACE_UNAVAILABLE")
	}
	return nil
}
````

## 第 48 步：spaceOwnerField

插入位置：第 47 步代码之后，紧接追加；完成稿约第 361 行。

本步声明：`func spaceOwnerField(kind string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind == &quot;ORGANIZATION&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;organization_id&quot;</code>；<code>&quot;account_owner_id&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>kind == &quot;ORGANIZATION&quot;</code>。条件成立时执行 <code>return &quot;organization_id&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;account_owner_id&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 48 bytes=19229:19360 -->
````go
func spaceOwnerField(kind string) string {
	if kind == "ORGANIZATION" {
		return "organization_id"
	}
	return "account_owner_id"
}
````

## 第 49 步：ExistingSpace

插入位置：第 48 步代码之后，紧接追加；完成稿约第 367 行。

本步声明：`func ExistingSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM space WHERE kind=? AND &quot;+spaceOwnerField(kind)+&quot;=? ORDER BY id LIMIT 1&quot;, kind, owner)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>spaceOwnerField(kind)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM space WHERE kind=? AND &quot;+spaceOwnerField(kind)+&quot;=? ORDER BY id LIMIT 1&quot;, kind, owner)</code> |
| <code>owner</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM space WHERE kind=? AND &quot;+spaceOwnerField(kind)+&quot;=? ORDER BY id LIMIT 1&quot;, kind, owner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM space WHERE kind=? AND &quot;+spaceOwnerField(kind)+&quot;=? ORDER BY id LIMIT 1&quot;, kind, owner).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM space WHERE kind=? AND &quot;+spaceOwnerField(kind)+&quot;=? ORDER BY id LIMIT 1&quot;, kind, owner)</code>。
- 此处通过 <code>return id, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 49 bytes=19360:19634 -->
````go
func ExistingSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (uint64, error) {
	var id uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM space WHERE kind=? AND "+spaceOwnerField(kind)+"=? ORDER BY id LIMIT 1", kind, owner).Scan(&id)
	return id, err
}
````

## 第 50 步：CreateSpace

插入位置：第 49 步代码之后，紧接追加；完成稿约第 372 行。

本步声明：`func CreateSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO space(kind,&quot;+spaceOwnerField(kind)+&quot;,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, kind, owner)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>spaceOwnerField(kind)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO space(kind,&quot;+spaceOwnerField(kind)+&quot;,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, kind, owner)</code> |
| <code>owner</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO space(kind,&quot;+spaceOwnerField(kind)+&quot;,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, kind, owner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO space(kind,&quot;+spaceOwnerField(kind)+&quot;,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, kind, owner)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 50 bytes=19634:19956 -->
````go
func CreateSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO space(kind,"+spaceOwnerField(kind)+",lifecycle) VALUES(?,?,'ACTIVE')", kind, owner)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return uint64(id), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/workspace_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
