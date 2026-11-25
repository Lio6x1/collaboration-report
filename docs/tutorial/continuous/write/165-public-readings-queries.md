# 分步编写 `services/document/internal/repository/document/public_readings_queries.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/165-public-readings-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取包含真实生命周期的正式发布权威快照。

公开留存与普通ACTIVE读取规则不同，需要保留关闭/留存事实供服务明确判断可见性。

写之前先按这条执行路线理解各段的连接：

1. PublicationFactsRow按document关联KB/space
2. lock=true以当前读在来源锁之后重取
3. scanPublicationDocument/KnowledgeBase/Space统一列和扫描顺序。

本文件需要守住的边界：

- 不能复用事务早期旧可见政策
- 查询层不自行判断可见，也不把未知文档当删除事件。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/public_readings_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/public_readings_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/public_readings_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/public_readings_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:326 -->
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

// public_facts.go：保留原实现，按业务合并文件。

````

## 第 2 步：PublicationFactsRow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

这里保留实际生命周期，供权威快照明确返回不可见状态；不放宽内部DocumentRow的ACTIVE要求。
lock=true的最终读取使用当前读，不能在等待来源锁后复用事务早期的可见策略。

本步声明：`func PublicationFactsRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.PublicationFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, 		&quot;SELECT &quot;+publicationDocumentColumns+&quot; FROM document WHERE id=?&quot;+suffix, id)</code>；<code>q.QueryRowContext(ctx, 			&quot;SELECT &quot;+publicationKnowledgeBaseColumns+&quot; FROM knowledge_base WHERE id=?&quot;+suffix, *d.KnowledgeBaseId)</code>；<code>q.QueryRowContext(ctx, 		&quot;SELECT &quot;+publicationSpaceColumns+&quot; FROM space WHERE id=?&quot;+suffix, d.SpaceId)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, 		&quot;SELECT &quot;+publicationDocumentColumns+&quot; FROM document WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>实际返回表达式：<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;DOCUMENT_NOT_FOUND&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_SPACE_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, 		&quot;SELECT &quot;+publicationDocumentColumns+&quot; FROM document WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return facts, domain.Violation(domain.EntityNotFound, &quot;DOCUMENT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 检查 <code>d.KnowledgeBaseId != nil</code>。
- 执行 <code>q.QueryRowContext(ctx, 			&quot;SELECT &quot;+publicationKnowledgeBaseColumns+&quot; FROM knowledge_base WHERE id=?&quot;+suffix, *d.KnowledgeBaseId)</code>。
- 检查 <code>err != nil &#124;&#124; kb.SpaceId != d.SpaceId</code>。条件成立时执行 <code>return facts, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, 		&quot;SELECT &quot;+publicationSpaceColumns+&quot; FROM space WHERE id=?&quot;+suffix, d.SpaceId)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_SPACE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=326:1877 -->
````go
// 这里保留实际生命周期，供权威快照明确返回不可见状态；不放宽内部DocumentRow的ACTIVE要求。
// lock=true的最终读取使用当前读，不能在等待来源锁后复用事务早期的可见策略。
func PublicationFactsRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.PublicationFacts, error) {
	var facts domain.PublicationFacts
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	document, sequence, err := scanPublicationDocument(q.QueryRowContext(ctx,
		"SELECT "+publicationDocumentColumns+" FROM document WHERE id=?"+suffix, id))
	if errors.Is(err, sql.ErrNoRows) {
		return facts, domain.Violation(domain.EntityNotFound, "DOCUMENT_NOT_FOUND")
	}
	if err != nil {
		return facts, err
	}
	facts.Document, facts.Sequence = document, sequence
	d := &facts.Document
	if d.KnowledgeBaseId != nil {
		kb, err := scanPublicationKnowledgeBase(q.QueryRowContext(ctx,
			"SELECT "+publicationKnowledgeBaseColumns+" FROM knowledge_base WHERE id=?"+suffix, *d.KnowledgeBaseId))
		if err != nil || kb.SpaceId != d.SpaceId {
			return facts, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
		}
		facts.KnowledgeBase = &kb
	}
	facts.Space, err = scanPublicationSpace(q.QueryRowContext(ctx,
		"SELECT "+publicationSpaceColumns+" FROM space WHERE id=?"+suffix, d.SpaceId))
	if err != nil {
		return facts, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SPACE_UNKNOWN")
	}
	return facts, nil
}

// public_facts_scan.go：保留原实现，按业务合并文件。

````

## 第 3 步：publicationDocumentColumns

插入位置：第 2 步代码之后，紧接追加；完成稿约第 50 行。

单项当前读和搜索批量快照共享列与扫描顺序，避免优化查询时漏掉权限或正式水位。
这里仅转换私有SQL结果，不判断ACTIVE/留存资格，也不创建或提交事务。

本步声明：`const publicationDocumentColumns = \`CAST(id AS CHAR),title,CAST(space_id AS CHAR),`。

<!-- write-step: 3 bytes=1877:2576 -->
````go
// 单项当前读和搜索批量快照共享列与扫描顺序，避免优化查询时漏掉权限或正式水位。
// 这里仅转换私有SQL结果，不判断ACTIVE/留存资格，也不创建或提交事务。
const publicationDocumentColumns = `CAST(id AS CHAR),title,CAST(space_id AS CHAR),
	CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,
	CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(current_manifest_id AS CHAR),
	CAST(current_release_id AS CHAR),publication_state,CAST(control_revision AS CHAR),
	CAST(acl_revision AS CHAR),CAST(publication_epoch AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR),event_seq`

````

## 第 4 步：publicationKnowledgeBaseColumns

插入位置：第 3 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`const publicationKnowledgeBaseColumns = \`CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,`。

<!-- write-step: 4 bytes=2576:2755 -->
````go
const publicationKnowledgeBaseColumns = `CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,
	CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR)`

````

## 第 5 步：publicationSpaceColumns

插入位置：第 4 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`const publicationSpaceColumns = \`CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),`。

<!-- write-step: 5 bytes=2755:2885 -->
````go
const publicationSpaceColumns = `CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),
	CAST(organization_id AS CHAR),lifecycle`

````

## 第 6 步：publicationScanner

插入位置：第 5 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`type publicationScanner interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scan</code> | <code>(...any) error</code> | 按SELECT列顺序传入接收变量的指针；返回解码/查询错误。抽象Row与Rows共用的扫描能力，不负责开启或提交事务。 |

<!-- write-step: 6 bytes=2885:2944 -->
````go
type publicationScanner interface {
	Scan(...any) error
}

````

## 第 7 步：scanPublicationDocument

插入位置：第 6 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func scanPublicationDocument(row publicationScanner) (dto.Document, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>publicationScanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Document</code> | 实际返回表达式：<code>d</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>sequence</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>row.Scan(&amp;d.Id, &amp;d.Title, &amp;d.SpaceId, &amp;d.KnowledgeBaseId, &amp;d.ResponsibleUserId, 		&amp;d.Lifecycle, &amp;d.MainLocked, &amp;structure, &amp;style, &amp;manifest, 		&amp;d.CurrentReleaseId, &amp;d.PublicationS …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return d, sequence, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2944:3771 -->
````go
func scanPublicationDocument(row publicationScanner) (dto.Document, uint64, error) {
	var d dto.Document
	var sequence uint64
	var structure, style, manifest sql.NullString
	err := row.Scan(&d.Id, &d.Title, &d.SpaceId, &d.KnowledgeBaseId, &d.ResponsibleUserId,
		&d.Lifecycle, &d.MainLocked, &structure, &style, &manifest,
		&d.CurrentReleaseId, &d.PublicationState, &d.ControlRevision, &d.AclRevision,
		&d.PublicationEpoch, &d.VisibilityPolicy, &d.VisibilityRevision, &sequence)
	// PURGED/初始化对象合法没有正文指针。公开与搜索先按真实生命周期拒绝，
	// 不能因NULL扫描失败把确定不可见变成依赖未知，也不补造任何固定版本号。
	d.CurrentStructureId, d.CurrentStyleId, d.CurrentManifestId = structure.String, style.String, manifest.String
	return d, sequence, err
}

````

## 第 8 步：scanPublicationKnowledgeBase

插入位置：第 7 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func scanPublicationKnowledgeBase(row publicationScanner) (dto.KnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>publicationScanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.KnowledgeBase</code> | 实际返回表达式：<code>kb</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>row.Scan(&amp;kb.Id, &amp;kb.SpaceId, &amp;kb.Name, &amp;kb.Lifecycle, &amp;kb.AclRevision, 		&amp;kb.VisibilityPolicy, &amp;kb.VisibilityRevision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return kb, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3771:4031 -->
````go
func scanPublicationKnowledgeBase(row publicationScanner) (dto.KnowledgeBase, error) {
	var kb dto.KnowledgeBase
	err := row.Scan(&kb.Id, &kb.SpaceId, &kb.Name, &kb.Lifecycle, &kb.AclRevision,
		&kb.VisibilityPolicy, &kb.VisibilityRevision)
	return kb, err
}

````

## 第 9 步：scanPublicationSpace

插入位置：第 8 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func scanPublicationSpace(row publicationScanner) (dto.Space, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>publicationScanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 实际返回表达式：<code>space</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>row.Scan(&amp;space.Id, &amp;space.Kind, &amp;space.OwnerUserId, &amp;space.OrganizationId, &amp;space.Lifecycle)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return space, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4031:4274 -->
````go
func scanPublicationSpace(row publicationScanner) (dto.Space, error) {
	var space dto.Space
	err := row.Scan(&space.Id, &space.Kind, &space.OwnerUserId, &space.OrganizationId, &space.Lifecycle)
	space.Actions = []string{}
	return space, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/public_readings_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
