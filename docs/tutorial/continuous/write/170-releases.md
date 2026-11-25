# 分步编写 `services/document/internal/repository/document/releases.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/170-releases.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取不可变正式版本和按ID倒序分页的发布历史。

历史版保留自己的标题与manifest；按数值ID游标可稳定跨页，不依赖可变展示时间。

写之前先按这条执行路线理解各段的连接：

1. FixedReleaseRow限定document/release
2. ReleaseRows使用验签后的before和limit，在原输出事务取一页。

本文件需要守住的边界：

- 知道release ID不等于有历史读取权限，资格由服务和当前来源锁提供。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/releases.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/releases.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/releases.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/releases.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:172 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
	"time"
)

````

## 第 2 步：FixedReleaseRow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func FixedReleaseRow(ctx context.Context, tx *sql.Tx, document, release uint64) (domain.FixedRelease, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS  …</code>（长表达式，按本步完整代码继续） |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.FixedRelease</code> | 使用 <code>domain.FixedRelease</code> 类型；其定义处的职责：定义发布意图、正式指针水位及PUBLIC/SPACE_MEMBERS/COLLABORATORS受众判定。<br>实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;RELEASE_NOT_FOUND&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.EntityNotFound, &quot;RELEASE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return fixed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=172:1089 -->
````go
func FixedReleaseRow(ctx context.Context, tx *sql.Tx, document, release uint64) (domain.FixedRelease, error) {
	var fixed domain.FixedRelease
	var created time.Time
	err := tx.QueryRowContext(ctx, "SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS CHAR),CAST(m.style_id AS CHAR) FROM `release` r JOIN main_manifest m ON m.id=r.manifest_id AND m.document_id=r.document_id WHERE r.document_id=? AND r.id=?", document, release).Scan(&fixed.Release.Id, &fixed.Release.DocumentId, &fixed.Release.ManifestId, &fixed.Release.PublisherId, &created, &fixed.Title, &fixed.Structure, &fixed.Style)
	if errors.Is(err, sql.ErrNoRows) {
		return fixed, domain.Violation(domain.EntityNotFound, "RELEASE_NOT_FOUND")
	}
	if err == nil {
		fixed.Release.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	}
	return fixed, err
}

````

## 第 3 步：ReleaseRows

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

不可变历史按数值ID倒序；游标边界和页大小由用例先验签，SQL只在原输出事务中取一页。

本步声明：`func ReleaseRows(ctx context.Context, tx *sql.Tx, document, before uint64, limit int) ([]dto.Release, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, statement, args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{document}</code> |
| <code>before</code> | <code>uint64</code> | 在此声明中的实际使用：<code>before != 0</code>；<code>append(args, before)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>append(args, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.Release</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>before != 0</code>。
- 执行 <code>tx.QueryContext(ctx, statement, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.Id, &amp;value.DocumentId, &amp;value.ManifestId, &amp;value.PublisherId, &amp;created)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1089:2129 -->
````go
// 不可变历史按数值ID倒序；游标边界和页大小由用例先验签，SQL只在原输出事务中取一页。
func ReleaseRows(ctx context.Context, tx *sql.Tx, document, before uint64, limit int) ([]dto.Release, error) {
	statement := "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(manifest_id AS CHAR),CAST(publisher_id AS CHAR),created_at FROM `release` WHERE document_id=?"
	args := []any{document}
	if before != 0 {
		statement += " AND id<?"
		args = append(args, before)
	}
	statement += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.Release{}
	for rows.Next() {
		var value dto.Release
		var created time.Time
		if err = rows.Scan(&value.Id, &value.DocumentId, &value.ManifestId, &value.PublisherId, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/releases.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
