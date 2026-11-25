# 分步编写 `services/account/internal/repository/account/output.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/055-output.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按输出来源类型重读Account当前权威代次。

已经构造的用户或组织JSON仍可能在发出前失去资格，须重新核对当时来源。

写之前先按这条执行路线理解各段的连接：

1. CurrentEpoch按identity/credential/session/org-member/org-admin/org-owner分支查询相应表。
2. 会话分支要求ACTIVE且未到期；组织分支要求成员和组织仍有效以及匹配角色。

本文件需要守住的边界：

- 无法识别的scope返回SOURCE_INVALID；查无当前资格不能继续释放响应。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/output.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/output.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/output.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/output.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:93 -->
````go
package account

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

````

## 第 2 步：Outputs

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`type Outputs struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=93:128 -->
````go
type Outputs struct{ db *sql.DB }

````

## 第 3 步：NewOutputs

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func NewOutputs(db *sql.DB) *Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Outputs</code> | 实际返回表达式：<code>&amp;Outputs{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Outputs{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=128:194 -->
````go
func NewOutputs(db *sql.DB) *Outputs { return &Outputs{db: db} }

````

## 第 4 步：Outputs.CurrentEpoch

插入位置：第 3 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`func (r *Outputs) CurrentEpoch(ctx context.Context, key string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(identity_epoch AS CHAR) FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27;&quot;, parts&#91;1&#93;)</code>；<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(credential_epoch AS CHAR) FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27;&quot;, parts&#91;1&#93;)</code>；<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(epoch AS CHAR) FROM session WHERE id=? AND state=&#x27;ACTIVE&#x27; AND expires_at&gt;UTC_TIMESTAMP(6)&quot;, id)</code>；<code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）<br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.Split(key, &quot;:&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SOURCE_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(identity_epoch AS CHAR) FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27;&quot;, parts&#91;1&#93;).Scan(&amp;value)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(identity_epoch AS CHAR) FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27;&quot;, parts&#91;1&#93;)</code>。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(credential_epoch AS CHAR) FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27;&quot;, parts&#91;1&#93;).Scan(&amp;value)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(credential_epoch AS CHAR) FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27;&quot;, parts&#91;1&#93;)</code>。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(epoch AS CHAR) FROM session WHERE id=? AND state=&#x27;ACTIVE&#x27; AND expires_at&gt;UTC_TIMESTAMP(6)&quot;, id).Scan(&amp;value)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(epoch AS CHAR) FROM session WHERE id=? AND state=&#x27;ACTIVE&#x27; AND expires_at&gt;UTC_TIMESTAMP(6)&quot;, id)</code>。
- 执行 <code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）。
- 执行 <code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）。
- 执行 <code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, ` 			SELECT CAST(m.epoch AS CHAR) FROM organization_member m 			JOIN organization o ON o.id=m.organization_id 			WHERE m.organization_id=? AND m.user_id=? …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return &quot;&quot;, errors.New(&quot;SOURCE_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=194:1879 -->
````go
func (r *Outputs) CurrentEpoch(ctx context.Context, key string) (string, error) {
	parts := strings.Split(key, ":")
	var value string
	var err error
	switch parts[0] {
	case "identity":
		err = r.db.QueryRowContext(ctx, "SELECT CAST(identity_epoch AS CHAR) FROM user WHERE id=? AND state='ACTIVE'", parts[1]).Scan(&value)
	case "credential":
		err = r.db.QueryRowContext(ctx, "SELECT CAST(credential_epoch AS CHAR) FROM user WHERE id=? AND state='ACTIVE'", parts[1]).Scan(&value)
	case "session":
		id, _ := hex.DecodeString(parts[1])
		err = r.db.QueryRowContext(ctx, "SELECT CAST(epoch AS CHAR) FROM session WHERE id=? AND state='ACTIVE' AND expires_at>UTC_TIMESTAMP(6)", id).Scan(&value)
	case "org-member":
		err = r.db.QueryRowContext(ctx, `
			SELECT CAST(m.epoch AS CHAR) FROM organization_member m
			JOIN organization o ON o.id=m.organization_id
			WHERE m.organization_id=? AND m.user_id=? AND m.state='ACTIVE' AND o.lifecycle='ACTIVE'`, parts[1], parts[2]).Scan(&value)
	case "org-admin":
		err = r.db.QueryRowContext(ctx, `
			SELECT CAST(m.epoch AS CHAR) FROM organization_member m
			JOIN organization o ON o.id=m.organization_id
			WHERE m.organization_id=? AND m.user_id=? AND m.state='ACTIVE'
			AND o.lifecycle='ACTIVE' AND m.role='ADMIN'`, parts[1], parts[2]).Scan(&value)
	case "org-owner":
		err = r.db.QueryRowContext(ctx, `
			SELECT CAST(m.epoch AS CHAR) FROM organization_member m
			JOIN organization o ON o.id=m.organization_id
			WHERE m.organization_id=? AND m.user_id=? AND m.state='ACTIVE'
			AND o.lifecycle='ACTIVE' AND o.owner_id=m.user_id`, parts[1], parts[2]).Scan(&value)
	default:
		return "", errors.New("SOURCE_INVALID")
	}
	return value, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/output.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
