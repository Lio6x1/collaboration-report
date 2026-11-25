# 分步编写 `services/account/internal/repository/account/organization_queries.go`

先按 [A03 · 组织、邀请申请、成员变更与组织关闭](../lessons/account/03-organizations.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/054-organization-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。

组织锁与成员锁保护同一次变更；关闭需先固定计划和屏障，再按Doc证据收尾。 成员搜索必须先在Account私有库内过滤昵称或规范账号，再按user_id游标取有界结果，不能只筛前端已加载的一页。

写之前先按这条执行路线理解各段的连接：

1. Applications.AdmitMember 在原tx恢复或新增MEMBER并递增组织member_revision。
2. PrepareMemberTransition/PrepareOwnerTransfer 进入REVOKING并增加epoch，InsertMemberTransition保存原计划。
3. SaveOrganizationCloseFence 校验并持久化document ACK，组织transition才可从FENCING到APPLIED；游标推进和最终CLOSED分别写入。
4. Organizations.Members先取可选search首项；调用方已trim/lower，SQL用LOCATE匹配LOWER(display_name)，另用去前导零后的账号作完整等值匹配。
5. WHERE先限定organization_id、非CLOSED、user_id>last和q，再ORDER BY user_id LIMIT；业务传limit+1探测，额外一项不返回给浏览器。

本文件需要守住的边界：

- 任务领取使用租约和generation，完成前必须重验ownership。
- ACK摘要冲突必须拒绝，不能靠覆盖数据库值让旧回执看似一致。
- search ...string是兼容原无搜索调用的可选单条件，只消费首项，不是把多个词当成OR条件。
- LOCATE把%和_当字面字符；所有用户值走占位符，不能拼接进SQL。搜索不改变ACTIVE/REVOKING状态或建库资格判定。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/organization_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/organization_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/organization_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/organization_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:416 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：Applications

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type Applications struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=416:456 -->
````go
type Applications struct{ db *sql.DB }

````

## 第 3 步：NewApplications

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewApplications(db *sql.DB) *Applications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Applications</code> | 实际返回表达式：<code>&amp;Applications{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Applications{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=456:537 -->
````go
func NewApplications(db *sql.DB) *Applications { return &Applications{db: db} }

````

## 第 4 步：Applications.Pending

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (r *Applications) Pending(ctx context.Context, tx *sql.Tx, organization, actor string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR) FROM organization_application 		WHERE organization_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; 		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR) FROM organization_application 		WHERE organization_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; 		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR) FROM organization_application 		WHERE organization_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; 		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR) FROM organization_application 		WHERE organization_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; 		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR) FROM organization_application 		WHERE organization_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; 		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor)</code>。
- 此处通过 <code>return id, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=537:902 -->
````go
func (r *Applications) Pending(ctx context.Context, tx *sql.Tx, organization, actor string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
		SELECT CAST(id AS CHAR) FROM organization_application
		WHERE organization_id=? AND applicant_id=? AND state='PENDING'
		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor).Scan(&id)
	return id, err
}

````

## 第 5 步：Applications.Insert

插入位置：第 4 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func (r *Applications) Insert(ctx context.Context, tx *sql.Tx, organization, actor, reason, invitation string) (int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application(organization_id,applicant_id,reason,state) 			VALUES(?,?,?,&#x27;PENDING&#x27;)`, organization, actor, reason)</code>；<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application 			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state) …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application(organization_id,applicant_id,reason,state) 			VALUES(?,?,?,&#x27;PENDING&#x27;)`, organization, actor, reason)</code>；<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application 			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state) …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application(organization_id,applicant_id,reason,state) 			VALUES(?,?,?,&#x27;PENDING&#x27;)`, organization, actor, reason)</code>；<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application 			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state) …</code>（长表达式，按本步完整代码继续） |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application(organization_id,applicant_id,reason,state) 			VALUES(?,?,?,&#x27;PENDING&#x27;)`, organization, actor, reason)</code>；<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application 			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state) …</code>（长表达式，按本步完整代码继续） |
| <code>invitation</code> | <code>string</code> | 在此声明中的实际使用：<code>invitation == &quot;&quot;</code>；<code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application 			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state) …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>invitation == &quot;&quot;</code>。
- 执行 <code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application(organization_id,applicant_id,reason,state) 			VALUES(?,?,?,&#x27;PENDING&#x27;)`, organization, actor, reason)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, ` 			INSERT INTO organization_application 			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state) …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=902:1661 -->
````go
func (r *Applications) Insert(ctx context.Context, tx *sql.Tx, organization, actor, reason, invitation string) (int64, error) {
	var result sql.Result
	var err error
	if invitation == "" {
		result, err = tx.ExecContext(ctx, `
			INSERT INTO organization_application(organization_id,applicant_id,reason,state)
			VALUES(?,?,?,'PENDING')`, organization, actor, reason)
	} else {
		result, err = tx.ExecContext(ctx, `
			INSERT INTO organization_application
			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state)
			VALUES(?,?,?,TRUE,UTC_TIMESTAMP(6),?,'PENDING')`, organization, actor, invitation, reason)
	}
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

````

## 第 6 步：Applications.Identity

插入位置：第 5 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func (r *Applications) Identity(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationApplication, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR) 		FROM organization_application WHERE id=?`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR) 		FROM organization_application WHERE id=?`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationApplication</code> | 使用 <code>domain.OrganizationApplication</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>application</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR) 		FROM organization_application WHERE id=?`, id).Scan(&amp;application.OrganizationID, &amp;application.ApplicantID)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR) 		FROM organization_application WHERE id=?`, id)</code>。
- 此处通过 <code>return application, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1661:2064 -->
````go
func (r *Applications) Identity(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationApplication, error) {
	var application domain.OrganizationApplication
	err := tx.QueryRowContext(ctx, `
		SELECT CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR)
		FROM organization_application WHERE id=?`, id).Scan(&application.OrganizationID, &application.ApplicantID)
	return application, err
}

````

## 第 7 步：Applications.LockStatus

插入位置：第 6 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func (r *Applications) LockStatus(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationApplication, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT state,CAST(revision AS CHAR) FROM organization_application 		WHERE id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT state,CAST(revision AS CHAR) FROM organization_application 		WHERE id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationApplication</code> | 使用 <code>domain.OrganizationApplication</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>application</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT state,CAST(revision AS CHAR) FROM organization_application 		WHERE id=? FOR UPDATE`, id).Scan(&amp;application.State, &amp;application.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT state,CAST(revision AS CHAR) FROM organization_application 		WHERE id=? FOR UPDATE`, id)</code>。
- 此处通过 <code>return application, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2064:2440 -->
````go
func (r *Applications) LockStatus(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationApplication, error) {
	var application domain.OrganizationApplication
	err := tx.QueryRowContext(ctx, `
		SELECT state,CAST(revision AS CHAR) FROM organization_application
		WHERE id=? FOR UPDATE`, id).Scan(&application.State, &application.Revision)
	return application, err
}

````

## 第 8 步：Applications.AdmitMember

插入位置：第 7 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func (r *Applications) AdmitMember(ctx context.Context, tx *sql.Tx, organization, applicant string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO organization_member(organization_id,user_id,role,state) 		  VALUES(?,?,&#x27;MEMBER&#x27;,&#x27;ACTIVE&#x27;) 		  ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,r …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO organization_member(organization_id,user_id,role,state) 		  VALUES(?,?,&#x27;MEMBER&#x27;,&#x27;ACTIVE&#x27;) 		  ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,r …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO organization_member(organization_id,user_id,role,state) 		  VALUES(?,?,&#x27;MEMBER&#x27;,&#x27;ACTIVE&#x27;) 		  ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,r …</code>（长表达式，按本步完整代码继续） |
| <code>applicant</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO organization_member(organization_id,user_id,role,state) 		  VALUES(?,?,&#x27;MEMBER&#x27;,&#x27;ACTIVE&#x27;) 		  ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,r …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO organization_member(organization_id,user_id,role,state) 		  VALUES(?,?,&#x27;MEMBER&#x27;,&#x27;ACTIVE&#x27;) 		  ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,r …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO organization_member(organization_id,user_id,role,state) 		  VALUES(?,?,&#x27;MEMBER&#x27;,&#x27;ACTIVE&#x27;) 		  ON DUPLICATE KEY UPDATE state=&#x27;AC …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2440:2922 -->
````go
func (r *Applications) AdmitMember(ctx context.Context, tx *sql.Tx, organization, applicant string) error {
	return execute(ctx, tx, []statement{
		{`INSERT INTO organization_member(organization_id,user_id,role,state)
		  VALUES(?,?,'MEMBER','ACTIVE')
		  ON DUPLICATE KEY UPDATE state='ACTIVE',role='MEMBER',epoch=epoch+1,revision=revision+1`, []any{organization, applicant}},
		{"UPDATE organization SET member_revision=member_revision+1 WHERE id=?", []any{organization}},
	})
}

````

## 第 9 步：Applications.Decide

插入位置：第 8 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (r *Applications) Decide(ctx context.Context, tx *sql.Tx, id, actor, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, actor, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, actor, id)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, actor, id)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, actor, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, actor, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2922:3174 -->
````go
func (r *Applications) Decide(ctx context.Context, tx *sql.Tx, id, actor, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?", state, actor, id)
	return err
}

````

## 第 10 步：Applications.List

插入位置：第 9 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func (r *Applications) List(ctx context.Context, actor, organization, last string, limit int) ([]domain.OrganizationApplication, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR), 		       reason,state,CAST(revision AS CHAR),CAST(source_invitation_id  …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>condition, key := &quot;applicant_id=?&quot;, actor</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>organization != &quot;&quot;</code>；<code>condition, key = &quot;organization_id=?&quot;, organization</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR), 		       reason,state,CAST(revision AS CHAR),CAST(source_invitation_id  …</code>（长表达式，按本步完整代码继续） |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR), 		       reason,state,CAST(revision AS CHAR),CAST(source_invitation_id  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationApplication</code> | 使用 <code>domain.OrganizationApplication</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>organization != &quot;&quot;</code>。
- 执行 <code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR), 		       reason,state,CAST(revision AS CHAR),CAST(source_invitation_id  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;application.ID, &amp;application.OrganizationID, &amp;application.ApplicantID, &amp;application.Reason, &amp;application.State, &amp;application.Revision, &amp;invitation, &amp;validated) != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;application.ID, &amp;application.OrganizationID, &amp;application.ApplicantID, &amp;application.Reason, &amp;application.State, &amp;application.Revision, &amp;invitation, &amp;validated)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>invitation.Valid</code>。
- 检查 <code>validated.Valid</code>。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return values, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3174:4703 -->
````go
func (r *Applications) List(ctx context.Context, actor, organization, last string, limit int) ([]domain.OrganizationApplication, error) {
	condition, key := "applicant_id=?", actor
	if organization != "" {
		condition, key = "organization_id=?", organization
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR),
		       reason,state,CAST(revision AS CHAR),CAST(source_invitation_id AS CHAR),invitation_validated_at
		FROM organization_application WHERE `+condition+" AND id>? ORDER BY id LIMIT ?", key, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationApplication{}
	for rows.Next() {
		var application domain.OrganizationApplication
		var invitation sql.NullString
		var validated sql.NullTime
		if rows.Scan(&application.ID, &application.OrganizationID, &application.ApplicantID, &application.Reason, &application.State, &application.Revision, &invitation, &validated) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		if invitation.Valid {
			application.SourceInvitationID = &invitation.String
		}
		if validated.Valid {
			application.InvitationValidatedAt = &validated.Time
		}
		values = append(values, application)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

// invitation.go：保留原实现，按业务合并文件。

````

## 第 11 步：Invitations

插入位置：第 10 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`type Invitations struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 11 bytes=4703:4742 -->
````go
type Invitations struct{ db *sql.DB }

````

## 第 12 步：NewInvitations

插入位置：第 11 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`func NewInvitations(db *sql.DB) *Invitations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Invitations</code> | 实际返回表达式：<code>&amp;Invitations{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Invitations{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4742:4820 -->
````go
func NewInvitations(db *sql.DB) *Invitations { return &Invitations{db: db} }

````

## 第 13 步：Invitations.Insert

插入位置：第 12 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`func (r *Invitations) Insert(ctx context.Context, tx *sql.Tx, organization, actor string, hash, encrypted []byte, expires time.Time) (int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续） |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续） |
| <code>encrypted</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续） |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO organization_invitation 		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at) 		VALUES(?,?,?,?,&#x27;dev-invitatio …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4820:5312 -->
````go
func (r *Invitations) Insert(ctx context.Context, tx *sql.Tx, organization, actor string, hash, encrypted []byte, expires time.Time) (int64, error) {
	row, err := tx.ExecContext(ctx, `
		INSERT INTO organization_invitation
		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at)
		VALUES(?,?,?,?,'dev-invitation-v1','OPEN',?)`, organization, actor, hash, encrypted, expires)
	if err != nil {
		return 0, err
	}
	id, _ := row.LastInsertId()
	return id, nil
}

````

## 第 14 步：Invitations.Share

插入位置：第 13 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func (r *Invitations) Share(ctx context.Context, id, organization string) ([]byte, time.Time, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, ` 		SELECT token_ciphertext, expires_at FROM organization_invitation 		WHERE id=? AND organization_id=?`, id, organization)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, ` 		SELECT token_ciphertext, expires_at FROM organization_invitation 		WHERE id=? AND organization_id=?`, id, organization)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, ` 		SELECT token_ciphertext, expires_at FROM organization_invitation 		WHERE id=? AND organization_id=?`, id, organization)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>encrypted</code> |
| <code>(匿名/未命名)</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>实际返回表达式：<code>expires</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, ` 		SELECT token_ciphertext, expires_at FROM organization_invitation 		WHERE id=? AND organization_id=?`, id, organization).Scan(&amp;encrypted, &amp;expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, ` 		SELECT token_ciphertext, expires_at FROM organization_invitation 		WHERE id=? AND organization_id=?`, id, organization)</code>。
- 此处通过 <code>return encrypted, expires, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5312:5680 -->
````go
func (r *Invitations) Share(ctx context.Context, id, organization string) ([]byte, time.Time, error) {
	var encrypted []byte
	var expires time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT token_ciphertext, expires_at FROM organization_invitation
		WHERE id=? AND organization_id=?`, id, organization).Scan(&encrypted, &expires)
	return encrypted, expires, err
}

````

## 第 15 步：Invitations.Organization

插入位置：第 14 步代码之后，紧接追加；完成稿约第 146 行。

本步声明：`func (r *Invitations) Organization(ctx context.Context, tx *sql.Tx, id string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(organization_id AS CHAR) FROM organization_invitation WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(organization_id AS CHAR) FROM organization_invitation WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>organization</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(organization_id AS CHAR) FROM organization_invitation WHERE id=?&quot;, id).Scan(&amp;organization)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(organization_id AS CHAR) FROM organization_invitation WHERE id=?&quot;, id)</code>。
- 此处通过 <code>return organization, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5680:5967 -->
````go
func (r *Invitations) Organization(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var organization string
	err := tx.QueryRowContext(ctx, "SELECT CAST(organization_id AS CHAR) FROM organization_invitation WHERE id=?", id).Scan(&organization)
	return organization, err
}

````

## 第 16 步：Invitations.LockStatus

插入位置：第 15 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`func (r *Invitations) LockStatus(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationInvitation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(revision AS CHAR) FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(revision AS CHAR) FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationInvitation</code> | 使用 <code>domain.OrganizationInvitation</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(revision AS CHAR) FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;value.State, &amp;value.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(revision AS CHAR) FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=5967:6310 -->
````go
func (r *Invitations) LockStatus(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationInvitation, error) {
	var value domain.OrganizationInvitation
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(revision AS CHAR) FROM organization_invitation WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Revision)
	return value, err
}

````

## 第 17 步：Invitations.Close

插入位置：第 16 步代码之后，紧接追加；完成稿约第 158 行。

本步声明：`func (r *Invitations) Close(ctx context.Context, tx *sql.Tx, id string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6310:6524 -->
````go
func (r *Invitations) Close(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_invitation SET state='CLOSED',revision=revision+1 WHERE id=?", id)
	return err
}

````

## 第 18 步：Invitations.IdentityByToken

插入位置：第 17 步代码之后，紧接追加；完成稿约第 163 行。

本步声明：`func (r *Invitations) IdentityByToken(ctx context.Context, tx *sql.Tx, hash []byte) (domain.InvitationIdentity, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR) 		FROM organization_invitation WHERE token_hash=?`, hash)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR) 		FROM organization_invitation WHERE token_hash=?`, hash)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.InvitationIdentity</code> | 使用 <code>domain.InvitationIdentity</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR) 		FROM organization_invitation WHERE token_hash=?`, hash).Scan(&amp;value.ID, &amp;value.OrganizationID)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR) 		FROM organization_invitation WHERE token_hash=?`, hash)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6524:6891 -->
````go
func (r *Invitations) IdentityByToken(ctx context.Context, tx *sql.Tx, hash []byte) (domain.InvitationIdentity, error) {
	var value domain.InvitationIdentity
	err := tx.QueryRowContext(ctx, `
		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR)
		FROM organization_invitation WHERE token_hash=?`, hash).Scan(&value.ID, &value.OrganizationID)
	return value, err
}

````

## 第 19 步：Invitations.LockValidity

插入位置：第 18 步代码之后，紧接追加；完成稿约第 171 行。

本步声明：`func (r *Invitations) LockValidity(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationInvitation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationInvitation</code> | 使用 <code>domain.OrganizationInvitation</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;value.State, &amp;value.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM organization_invitation WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=6891:7223 -->
````go
func (r *Invitations) LockValidity(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationInvitation, error) {
	var value domain.OrganizationInvitation
	err := tx.QueryRowContext(ctx, "SELECT state,expires_at FROM organization_invitation WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Expires)
	return value, err
}

````

## 第 20 步：Invitations.List

插入位置：第 19 步代码之后，紧接追加；完成稿约第 177 行。

本步声明：`func (r *Invitations) List(ctx context.Context, organization, last string, limit int) ([]domain.OrganizationInvitation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR) 		FROM organization_invitation WHERE organization_id=? AND id&gt;? 		ORDER BY id LIMIT ?`, organization, last, limit)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR) 		FROM organization_invitation WHERE organization_id=? AND id&gt;? 		ORDER BY id LIMIT ?`, organization, last, limit)</code>；<code>OrganizationID: organization</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR) 		FROM organization_invitation WHERE organization_id=? AND id&gt;? 		ORDER BY id LIMIT ?`, organization, last, limit)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR) 		FROM organization_invitation WHERE organization_id=? AND id&gt;? 		ORDER BY id LIMIT ?`, organization, last, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationInvitation</code> | 使用 <code>domain.OrganizationInvitation</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, ` 		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR) 		FROM organization_invitation WHERE organization_id=? AND id&gt;? 		ORDER BY id LIMIT ?`, organization, last, limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;value.ID, &amp;value.State, &amp;value.Expires, &amp;value.Revision) != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.ID, &amp;value.State, &amp;value.Expires, &amp;value.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return values, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=7223:8205 -->
````go
func (r *Invitations) List(ctx context.Context, organization, last string, limit int) ([]domain.OrganizationInvitation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR)
		FROM organization_invitation WHERE organization_id=? AND id>?
		ORDER BY id LIMIT ?`, organization, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationInvitation{}
	for rows.Next() {
		value := domain.OrganizationInvitation{OrganizationID: organization}
		if rows.Scan(&value.ID, &value.State, &value.Expires, &value.Revision) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 21 步：OrganizationCloseRecoveryKind

插入位置：第 20 步代码之后，紧接追加；完成稿约第 202 行。

本步声明：`const OrganizationCloseRecoveryKind = "ORG_CLOSE_RECOVER"`。

<!-- write-step: 21 bytes=8205:8264 -->
````go
const OrganizationCloseRecoveryKind = "ORG_CLOSE_RECOVER"

````

## 第 22 步：OrganizationCloseRecord

插入位置：第 21 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`type OrganizationCloseRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Organization</code> | <code>uint64</code> | 要关闭的组织编号。 |
| <code>Actor</code> | <code>uint64</code> | 发起关闭的原所有者账号。 |
| <code>State</code> | <code>string</code> | 组织关闭事务流程当前状态。 |
| <code>Plan</code> | <code>&#91;&#93;byte</code> | 原固定计划JSON，恢复逐步沿用，不重新枚举后替换。 |

<!-- write-step: 22 bytes=8264:8389 -->
````go
type OrganizationCloseRecord struct {
	Organization, Actor uint64
	State               string
	Plan                []byte
}

````

## 第 23 步：ReadOrganizationClose

插入位置：第 22 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`func ReadOrganizationClose(ctx context.Context, q Queryer, operation string, lock bool) (OrganizationCloseRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT organization_id,actor_id,state,fixed_plan FROM organization_transition WHERE operation_id=? AND kind=&#x27;CLOSE&#x27;&quot;+suffix, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT organization_id,actor_id,state,fixed_plan FROM organization_transition WHERE operation_id=? AND kind=&#x27;CLOSE&#x27;&quot;+suffix, operation)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OrganizationCloseRecord</code> | 实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT organization_id,actor_id,state,fixed_plan FROM organization_transition WHERE operation_id=? AND kind=&#x27;CLOSE&#x27;&quot;+suffix, operation).Scan(&amp;record.Organiz …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT organization_id,actor_id,state,fixed_plan FROM organization_transition WHERE operation_id=? AND kind=&#x27;CLOSE&#x27;&quot;+suffix, operation)</code>。
- 此处通过 <code>return record, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=8389:8863 -->
````go
func ReadOrganizationClose(ctx context.Context, q Queryer, operation string, lock bool) (OrganizationCloseRecord, error) {
	var record OrganizationCloseRecord
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT organization_id,actor_id,state,fixed_plan FROM organization_transition WHERE operation_id=? AND kind='CLOSE'"+suffix, operation).Scan(&record.Organization, &record.Actor, &record.State, &record.Plan)
	return record, err
}

````

## 第 24 步：BeginOrganizationClose

插入位置：第 23 步代码之后，紧接追加；完成稿约第 220 行。

Begin 在已持 user/session/receipt 锁后锁组织及全部成员；整个关闭意图同事务提交。

本步声明：`func BeginOrganizationClose(ctx context.Context, tx *sql.Tx, operation, organization, actor, expected string) (domain.OrganizationClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ReadOrganization(ctx, tx, organization, true)</code>；<code>tx.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,epoch FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE&quot;, organization)</code>；<code>tx.ExecContext(ctx, `INSERT INTO organization_transition(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 	 VALUES(?,?,?,?,&#x27;CLOSE&#x27;,?,&#x27;F …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE organization SET lifecycle=&#x27;CLOSING&#x27;,revision=revision+1,member_revision=member_revision+1 WHERE id=?&quot;, orgID)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ReadOrganization(ctx, tx, organization, true)</code>；<code>tx.QueryContext</code>；<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>Operation: operation</code>；<code>tx.ExecContext(ctx, `INSERT INTO organization_transition(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 	 VALUES(?,?,?,?,&#x27;CLOSE&#x27;,?,&#x27;F …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) 	 ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) 	 VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;docu …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>ReadOrganization(ctx, tx, organization, true)</code>；<code>tx.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,epoch FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE&quot;, organization)</code>；<code>strconv.ParseUint(organization, 10, 64)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>member.UserID == actor</code>；<code>domain.OrganizationCloseEligible(org, actor, expected, own, live)</code>；<code>strconv.ParseUint(actor, 10, 64)</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OrganizationCloseEligible(org, actor, expected, own, live)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。<br>实际返回表达式：<code>empty</code>；<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT CAST(user_id AS CHAR),role,state,epoch FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE&quot;, organization)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;member.UserID, &amp;member.Role, &amp;member.State, &amp;member.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>member.State != &quot;CLOSED&quot;</code>。
- 检查 <code>member.UserID == actor</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!plan.Binding.Valid() &#124;&#124; revision == ^uint64(0) &#124;&#124; members == ^uint64(0)</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=8863:10607 -->
````go
// Begin 在已持 user/session/receipt 锁后锁组织及全部成员；整个关闭意图同事务提交。
func BeginOrganizationClose(ctx context.Context, tx *sql.Tx, operation, organization, actor, expected string) (domain.OrganizationClosePlan, error) {
	var empty domain.OrganizationClosePlan
	org, err := ReadOrganization(ctx, tx, organization, true)
	if err != nil {
		return empty, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT CAST(user_id AS CHAR),role,state,epoch FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE", organization)
	if err != nil {
		return empty, err
	}
	live := 0
	var own domain.OrganizationMember
	for rows.Next() {
		var member domain.OrganizationMember
		if err = rows.Scan(&member.UserID, &member.Role, &member.State, &member.Epoch); err != nil {
			break
		}
		if member.State != "CLOSED" {
			live++
		}
		if member.UserID == actor {
			own = member
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return empty, err
	}
	if err = domain.OrganizationCloseEligible(org, actor, expected, own, live); err != nil {
		return empty, err
	}
	orgID, _ := strconv.ParseUint(organization, 10, 64)
	user, _ := strconv.ParseUint(actor, 10, 64)
	revision, _ := strconv.ParseUint(org.Revision, 10, 64)
	members, _ := strconv.ParseUint(org.MemberRevision, 10, 64)
	plan := domain.OrganizationClosePlan{Binding: platform.OrganizationClosureBinding{Operation: operation, Organization: orgID, User: user, Revision: revision, MemberRevision: members, Epoch: own.Epoch}}
	if !plan.Binding.Valid() || revision == ^uint64(0) || members == ^uint64(0) {
		return empty, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
````

## 第 25 步：BeginOrganizationClose（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 265 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO organization_transition(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 	 VALUES(?,?,?,?,&#x27;CLOSE&#x27;,?,&#x27;F …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization SET lifecycle=&#x27;CLOSING&#x27;,revision=revision+1,member_revision=member_revision+1 WHERE id=?&quot;, orgID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1 WHERE organization_id=? AND user_id=?&quot;, orgID, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) 	 ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id SET i.state=&#x27;CLOSED&#x27; WHERE s.scope_key=? AND s.epoch&lt;=?&quot;, scope, own.Epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) 	 VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;docu …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(?,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, OrganizationCloseRecoveryKind, operation, operation, string(raw))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=10607:12526 -->
````go
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_transition(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
	 VALUES(?,?,?,?,'CLOSE',?,'FENCING',?)`, operation, orgID, user, user, revision, string(raw)); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE organization SET lifecycle='CLOSING',revision=revision+1,member_revision=member_revision+1 WHERE id=?", orgID); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1 WHERE organization_id=? AND user_id=?", orgID, user); err != nil {
		return empty, err
	}
	scope := plan.Binding.Scope()
	if _, err = tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?)
	 ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1`, scope, own.Epoch, operation); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=?", scope, own.Epoch); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at)
	 VALUES(?,?,?,?,?,'FENCING','["document"]',UTC_TIMESTAMP(6))`, operation, user, scope, own.Epoch, string(raw)); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, OrganizationCloseRecoveryKind, operation, operation, string(raw)); err != nil {
		return empty, err
	}
	return plan, nil
}

````

## 第 26 步：ClaimOrganizationClose

插入位置：第 25 步代码之后，紧接追加；完成稿约第 296 行。

本步声明：`func ClaimOrganizationClose(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{OrganizationCloseRecoveryKind})</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+accountCloseFenceTaskColumns+&quot; FROM durable_task t WHERE id=? AND kind=? FOR UPDATE&quot;, claimed.ID, OrganizationCloseRecoveryKind)</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{OrganizationCloseRecoveryKind})</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance == &quot;&quot;</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）；<code>task.State, task.Instance, task.LeaseValid = &quot;RUNNING&quot;, instance, true</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>domain.AccountCloseFenceTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_CLAIM_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.AccountCloseFenceTask{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+accountCloseFenceTaskColumns+&quot; FROM durable_task t WHERE id=? AND kind=? FOR UPDATE&quot;, claimed.ID, OrganizationCloseRecoveryKind)</code>。
- 检查 <code>err != nil &#124;&#124; instance == &quot;&quot;</code>。条件成立时执行 <code>return task, errors.New(&quot;ORGANIZATION_CLOSE_CLAIM_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 此处通过 <code>return task, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=12526:13662 -->
````go
func ClaimOrganizationClose(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	claimed, err := claimClosureTask(ctx, tx, []string{OrganizationCloseRecoveryKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, "SELECT "+accountCloseFenceTaskColumns+" FROM durable_task t WHERE id=? AND kind=? FOR UPDATE", claimed.ID, OrganizationCloseRecoveryKind))
	if err != nil || instance == "" {
		return task, errors.New("ORGANIZATION_CLOSE_CLAIM_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
	 WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, OrganizationCloseRecoveryKind, task.Generation)
	if err = accountCloseChanged(result, err); err != nil {
		return task, err
	}
	task.Generation++
	task.State, task.Instance, task.LeaseValid = "RUNNING", instance, true
	return task, nil
}

````

## 第 27 步：ValidateOrganizationCloseTask

插入位置：第 26 步代码之后，紧接追加；完成稿约第 316 行。

本步声明：`func ValidateOrganizationCloseTask(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+accountCloseFenceTaskColumns+&quot; FROM durable_task t WHERE id=? AND kind=? FOR UPDATE&quot;, task.ID, OrganizationCloseRecoveryKind)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Generation</code>；<code>task.Instance</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_OWNERSHIP_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+accountCloseFenceTaskColumns+&quot; FROM durable_task t WHERE id=? AND kind=? FOR UPDATE&quot;, task.ID, OrganizationCloseRecoveryKind)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.Operation != task.Operation &#124;&#124; current.Revocation != task.Operation &#124;&#124; current.Generation != task.Generation &#124;&#124; current.Instance != task.Instance &#124;&#124; current.State != &quot;RUNNI …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_OWNERSHIP_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=13662:14348 -->
````go
func ValidateOrganizationCloseTask(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask) error {
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, "SELECT "+accountCloseFenceTaskColumns+" FROM durable_task t WHERE id=? AND kind=? FOR UPDATE", task.ID, OrganizationCloseRecoveryKind))
	if err != nil {
		return err
	}
	if current.Operation != task.Operation || current.Revocation != task.Operation || current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ORGANIZATION_CLOSE_OWNERSHIP_CHANGED")
	}
	return nil
}

````

## 第 28 步：RetryOrganizationClose

插入位置：第 27 步代码之后，紧接追加；完成稿约第 327 行。

本步声明：`func RetryOrganizationClose(ctx context.Context, db *sql.DB, task domain.AccountCloseFenceTask, code string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) 	 WHERE id=? AND kin …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) 	 WHERE id=? AND kin …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) 	 WHERE id=? AND kin …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 28 bytes=14348:14789 -->
````go
func RetryOrganizationClose(ctx context.Context, db *sql.DB, task domain.AccountCloseFenceTask, code string) {
	_, _ = db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING'`, code, task.ID, OrganizationCloseRecoveryKind, task.Generation, task.Instance)
}

````

## 第 29 步：FinishOrganizationCloseTask

插入位置：第 28 步代码之后，紧接追加；完成稿约第 332 行。

本步声明：`func FinishOrganizationCloseTask(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, terminal bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6) 	 WHERE id=? AND kind=? AND generation=? AND instan …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>terminal</code> | <code>bool</code> | true表示恢复已到终态，任务置SUCCEEDED；false表示本轮仅推进了一部分，重新置QUEUED。两种情况都按任务ID、代次、实例与有效租约条件写回。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>accountCloseChanged(result, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>terminal</code>。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6) 	 WHERE id=? AND kind=? AND generation=? AND instan …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=14789:15354 -->
````go
func FinishOrganizationCloseTask(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, terminal bool) error {
	state := "QUEUED"
	if terminal {
		state = "SUCCEEDED"
	}
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, state, task.ID, OrganizationCloseRecoveryKind, task.Generation, task.Instance)
	return accountCloseChanged(result, err)
}

````

## 第 30 步：OrganizationCloseProof

插入位置：第 29 步代码之后，紧接追加；完成稿约第 342 行。

Proof核验原授权、gate、固定目标与真实Doc ACK；单纯组织CLOSING不能授予清理资格。

本步声明：`func OrganizationCloseProof(ctx context.Context, q Queryer, binding platform.OrganizationClosureBinding, requireACK bool) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT a.actor_id,a.state,a.scope_key,a.epoch,a.target_permission,a.participant_plan,g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;) 	 FROM authorizat …</code>（长表达式，按本步完整代码继续）；<code>q.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;`, binding.Operation, scope, epoch)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>binding</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>binding.Operation</code>；<code>plan.Binding != binding</code>；<code>binding.User</code>；<code>binding.Scope</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>requireACK</code> | <code>bool</code> | 在此声明中的实际使用：<code>!requireACK</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>state == &quot;FENCING&quot;</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_PROOF_CHANGED&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_ACK_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT a.actor_id,a.state,a.scope_key,a.epoch,a.target_permission,a.participant_plan,g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;) 	 FROM authorizat …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT a.actor_id,a.state,a.scope_key,a.epoch,a.target_permission,a.participant_plan,g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;) 	 FROM authorizat …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; plan.Binding != binding &#124;&#124; actor != binding.User &#124;&#124; scope != binding.Scope() &#124;&#124; epoch != binding.Epoch &#124;&#124; maxEpoch != epoch &#124;&#124; gateState != &quot;BLOCKED&quot; &#124;&#124; revocation != …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_PROOF_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!requireACK</code>。条件成立时执行 <code>return state == &quot;FENCING&quot;, nil</code>。当前函数在这里结束。
- 检查 <code>state != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;`, binding.Operation, scope, epoch).Scan(&amp;digest)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;`, binding.Operation, scope, epoch)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest, platform.BootstrapDigest(request, &quot;document&quot;))</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_ACK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=15354:17249 -->
````go
// Proof核验原授权、gate、固定目标与真实Doc ACK；单纯组织CLOSING不能授予清理资格。
func OrganizationCloseProof(ctx context.Context, q Queryer, binding platform.OrganizationClosureBinding, requireACK bool) (bool, error) {
	var state, scope, gateState, revocation string
	var actor, epoch, maxEpoch uint64
	var target, participants []byte
	err := q.QueryRowContext(ctx, `SELECT a.actor_id,a.state,a.scope_key,a.epoch,a.target_permission,a.participant_plan,g.max_epoch,g.state,COALESCE(g.last_revocation_id,'')
	 FROM authorization_change a JOIN access_gate g ON g.scope_key=a.scope_key WHERE a.revocation_id=?`, binding.Operation).Scan(&actor, &state, &scope, &epoch, &target, &participants, &maxEpoch, &gateState, &revocation)
	if err != nil {
		return false, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(target)
	var peers []string
	if err != nil || plan.Binding != binding || actor != binding.User || scope != binding.Scope() || epoch != binding.Epoch || maxEpoch != epoch || gateState != "BLOCKED" || revocation != binding.Operation || json.Unmarshal(participants, &peers) != nil || !slices.Equal(peers, []string{"document"}) {
		return false, errors.New("ORGANIZATION_CLOSE_PROOF_CHANGED")
	}
	if !requireACK {
		return state == "FENCING", nil
	}
	if state != "SUCCEEDED" {
		return false, nil
	}
	var digest []byte
	err = q.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'`, binding.Operation, scope, epoch).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	request := OrganizationCloseFenceRequest(binding)
	if !bytes.Equal(digest, platform.BootstrapDigest(request, "document")) {
		return false, errors.New("ORGANIZATION_CLOSE_ACK_CHANGED")
	}
	return true, nil
}

````

## 第 31 步：OrganizationCloseFenceRequest

插入位置：第 30 步代码之后，紧接追加；完成稿约第 378 行。

本步声明：`func OrganizationCloseFenceRequest(b platform.OrganizationClosureBinding) *pb.FenceRequest`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Operation</code>；<code>b.Scope</code>；<code>b.Epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceRequest</code> | 实际返回表达式：<code>&amp;pb.FenceRequest{RevocationId: b.Operation, ClosedScopes: &#91;&#93;*pb.SourceEpoch{{ScopeKey: b.Scope(), Epoch: b.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(b.Scope(), b.Epoch)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.FenceRequest{RevocationId: b.Operation, ClosedScopes: &#91;&#93;*pb.SourceEpoch{{ScopeKey: b.Scope(), Epoch: b.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(b.Scope(), b.Epoch)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=17249:17548 -->
````go
func OrganizationCloseFenceRequest(b platform.OrganizationClosureBinding) *pb.FenceRequest {
	return &pb.FenceRequest{RevocationId: b.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: b.Scope(), Epoch: b.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(b.Scope(), b.Epoch)}
}

````

## 第 32 步：SaveOrganizationCloseFence

插入位置：第 31 步代码之后，紧接追加；完成稿约第 382 行。

本步声明：`func SaveOrganizationCloseFence(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, digest []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) 	 VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIME …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;`, b.Operation, b.Scope(), b.Epoch)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27;,retry_at=NULL,last_error=NULL WHERE revocation_id=? AND state=&#x27;FENCING&#x27;&quot;, b.Operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET state=&#x27;APPLIED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;CLOSE&#x27; AND state=&#x27;FENCING&#x27;&quot;, b.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>OrganizationCloseFenceRequest(b)</code>；<code>b.Operation</code>；<code>b.Scope</code>；<code>b.Epoch</code> |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.Equal(digest, platform.BootstrapDigest(OrganizationCloseFenceRequest(b), &quot;document&quot;))</code>；<code>tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) 	 VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIME …</code>（长表达式，按本步完整代码继续）；<code>bytes.Equal(prior, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_ACK_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_ACK_CONFLICT&quot;)</code>；<code>accountCloseChanged(result, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(digest, platform.BootstrapDigest(OrganizationCloseFenceRequest(b), &quot;document&quot;))</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_ACK_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) 	 VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIME …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;`, b.Operation, b.Scope(), b.Epoch).Scan(&amp;prior)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;`, b.Operation, b.Scope(), b.Epoch)</code>。
- 检查 <code>!bytes.Equal(prior, digest)</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_ACK_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27;,retry_at=NULL,last_error=NULL WHERE revocation_id=? AND state=&#x27;FENCING&#x27;&quot;, b.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET state=&#x27;APPLIED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;CLOSE&#x27; AND state=&#x27;FENCING&#x27;&quot;, b.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=17548:18963 -->
````go
func SaveOrganizationCloseFence(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, digest []byte) error {
	if !bytes.Equal(digest, platform.BootstrapDigest(OrganizationCloseFenceRequest(b), "document")) {
		return errors.New("ORGANIZATION_CLOSE_ACK_INVALID")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at)
	 VALUES(?,'document',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE revocation_id=revocation_id`, b.Operation, b.Scope(), b.Epoch, digest); err != nil {
		return err
	}
	var prior []byte
	if err := tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'`, b.Operation, b.Scope(), b.Epoch).Scan(&prior); err != nil {
		return err
	}
	if !bytes.Equal(prior, digest) {
		return errors.New("ORGANIZATION_CLOSE_ACK_CONFLICT")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE authorization_change SET state='SUCCEEDED',retry_at=NULL,last_error=NULL WHERE revocation_id=? AND state='FENCING'", b.Operation); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE organization_transition SET state='APPLIED',generation=generation+1 WHERE operation_id=? AND kind='CLOSE' AND state='FENCING'", b.Operation)
	return accountCloseChanged(result, err)
}

````

## 第 33 步：SaveOrganizationCloseCursor

插入位置：第 32 步代码之后，紧接追加；完成稿约第 404 行。

本步声明：`func SaveOrganizationCloseCursor(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=&#x27;CLOSE&#x27; AND state=&#x27;APPLIED&#x27;&quot;, string(raw), plan.Binding.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。<br>在此声明中的实际使用：<code>json.Marshal(plan)</code>；<code>plan.Binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>accountCloseChanged(result, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=&#x27;CLOSE&#x27; AND state=&#x27;APPLIED&#x27;&quot;, string(raw), plan.Binding.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=18963:19387 -->
````go
func SaveOrganizationCloseCursor(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE organization_transition SET fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind='CLOSE' AND state='APPLIED'", string(raw), plan.Binding.Operation)
	return accountCloseChanged(result, err)
}

````

## 第 34 步：CompleteOrganizationClose

插入位置：第 33 步代码之后，紧接追加；完成稿约第 413 行。

本步声明：`func CompleteOrganizationClose(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, statements)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, statements)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Organization</code>；<code>b.User</code>；<code>b.Epoch</code>；<code>b.Operation</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, statements)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, statements)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=19387:20661 -->
````go
func CompleteOrganizationClose(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, result []byte) error {
	statements := []statement{
		{"UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=? AND state='REVOKING' AND epoch=?", []any{b.Organization, b.User, b.Epoch + 1}},
		{"UPDATE organization SET lifecycle='CLOSED',closed_at=UTC_TIMESTAMP(6) WHERE id=? AND lifecycle='CLOSING'", []any{b.Organization}},
		{"UPDATE organization_invitation SET state='CLOSED' WHERE organization_id=? AND state='OPEN'", []any{b.Organization}},
		{"UPDATE organization_application SET state='CLOSED' WHERE organization_id=? AND state='OPEN'", []any{b.Organization}},
		{"UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND state<>'CLOSED'", []any{b.Organization}},
		{"UPDATE organization_transition SET state='SUCCEEDED',generation=generation+1 WHERE operation_id=? AND kind='CLOSE' AND state='APPLIED'", []any{b.Operation}},
		{"UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6),generation=generation+1 WHERE operation_id=? AND kind='ORG_CLOSE' AND state='PREPARING'", []any{string(result), b.Operation}},
	}
	return execute(ctx, tx, statements)
}

````

## 第 35 步：OrganizationCloseReady

插入位置：第 34 步代码之后，紧接追加；完成稿约第 426 行。

Ready全量分页检查本流程未终结任务，既不接管账号任务，也不靠lease过期猜终态。

本步声明：`func OrganizationCloseReady(ctx context.Context, db *sql.DB) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT t.id,t.operation_id,t.business_key,t.payload,c.organization_id,c.actor_id,c.state,c.fixed_plan,COALESCE(o.kind,&#x27;&#x27;),COALESCE(o.state,&#x27;&#x27;) 		 FROM durable …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT t.id,t.operation_id,t.business_key,t.payload,c.organization_id,c.actor_id,c.state,c.fixed_plan,COALESCE(o.kind,&#x27;&#x27;),COALESCE(o.state,&#x27;&#x27;) 		 FROM durable …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;operation, &amp;key, &amp;payload, &amp;org, &amp;user, &amp;state, &amp;raw, &amp;kind, &amp;receipt)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>pErr != nil &#124;&#124; cErr != nil &#124;&#124; p.Binding != c.Binding &#124;&#124; p.Binding.Operation != operation &#124;&#124; key != operation &#124;&#124; !org.Valid &#124;&#124; uint64(org.Int64) != p.Binding.Organization &#124;&#124; !user.V …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count &lt; 100</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 35 bytes=20661:22433 -->
````go
// Ready全量分页检查本流程未终结任务，既不接管账号任务，也不靠lease过期猜终态。
func OrganizationCloseReady(ctx context.Context, db *sql.DB) error {
	var after uint64
	for {
		rows, err := db.QueryContext(ctx, `SELECT t.id,t.operation_id,t.business_key,t.payload,c.organization_id,c.actor_id,c.state,c.fixed_plan,COALESCE(o.kind,''),COALESCE(o.state,'')
		 FROM durable_task t LEFT JOIN organization_transition c ON c.operation_id=t.operation_id AND c.kind='CLOSE'
		 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, OrganizationCloseRecoveryKind, after)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var id uint64
			var operation, key, state, kind, receipt string
			var org, user sql.NullInt64
			var payload, raw []byte
			if err = rows.Scan(&id, &operation, &key, &payload, &org, &user, &state, &raw, &kind, &receipt); err != nil {
				break
			}
			p, pErr := domain.DecodeOrganizationClosePlan(payload)
			c, cErr := domain.DecodeOrganizationClosePlan(raw)
			if pErr != nil || cErr != nil || p.Binding != c.Binding || p.Binding.Operation != operation || key != operation || !org.Valid || uint64(org.Int64) != p.Binding.Organization || !user.Valid || uint64(user.Int64) != p.Binding.User || (state != "FENCING" && state != "APPLIED") || kind != "ORG_CLOSE" || receipt != "PREPARING" {
				err = errors.New("ORGANIZATION_CLOSE_TASK_INVALID")
				break
			}
			after, count = id, count+1
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

// organization_read.go：保留原实现，按业务合并文件。

````

## 第 36 步：Organizations

插入位置：第 35 步代码之后，紧接追加；完成稿约第 468 行。

本步声明：`type Organizations struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 36 bytes=22433:22474 -->
````go
type Organizations struct{ db *sql.DB }

````

## 第 37 步：NewOrganizations

插入位置：第 36 步代码之后，紧接追加；完成稿约第 470 行。

本步声明：`func NewOrganizations(db *sql.DB) *Organizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Organizations</code> | 实际返回表达式：<code>&amp;Organizations{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Organizations{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=22474:22558 -->
````go
func NewOrganizations(db *sql.DB) *Organizations { return &Organizations{db: db} }

````

## 第 38 步：ReadOrganization

插入位置：第 37 步代码之后，紧接追加；完成稿约第 472 行。

本步声明：`func ReadOrganization(ctx context.Context, query Queryer, id string, lock bool) (domain.Organization, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>query.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),name,CAST(owner_id AS CHAR),lifecycle,CAST(revision AS CHAR),CAST(member_revision AS CHAR) FROM organization WHERE id=?&quot;+suffix, id)</code> |
| <code>query</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>query.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>query.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),name,CAST(owner_id AS CHAR),lifecycle,CAST(revision AS CHAR),CAST(member_revision AS CHAR) FROM organization WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Organization</code> | 使用 <code>domain.Organization</code> 类型；其定义处的职责：区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>query.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),name,CAST(owner_id AS CHAR),lifecycle,CAST(revision AS CHAR),CAST(member_revision AS CHAR) FROM organization WHERE id=?&quot;+suffix, …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>query.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),name,CAST(owner_id AS CHAR),lifecycle,CAST(revision AS CHAR),CAST(member_revision AS CHAR) FROM organization WHERE id=?&quot;+suffix, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=22558:23071 -->
````go
func ReadOrganization(ctx context.Context, query Queryer, id string, lock bool) (domain.Organization, error) {
	var value domain.Organization
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := query.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),name,CAST(owner_id AS CHAR),lifecycle,CAST(revision AS CHAR),CAST(member_revision AS CHAR) FROM organization WHERE id=?"+suffix, id).Scan(&value.ID, &value.Name, &value.OwnerID, &value.Lifecycle, &value.Revision, &value.MemberRevision)
	return value, err
}

````

## 第 39 步：ReadOrganizationMember

插入位置：第 38 步代码之后，紧接追加；完成稿约第 482 行。

本步声明：`func ReadOrganizationMember(ctx context.Context, query Queryer, organization, actor string, lock bool) (domain.OrganizationMember, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>query.QueryRowContext(ctx, &quot;SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?&quot;+suffix, organization, actor)</code> |
| <code>query</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>query.QueryRowContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>query.QueryRowContext(ctx, &quot;SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?&quot;+suffix, organization, actor)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>query.QueryRowContext(ctx, &quot;SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?&quot;+suffix, organization, actor)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationMember</code> | 使用 <code>domain.OrganizationMember</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>query.QueryRowContext(ctx, &quot;SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?&quot;+suffix, organization, actor).Scan(&amp;value. …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>query.QueryRowContext(ctx, &quot;SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?&quot;+suffix, organization, actor)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=23071:23562 -->
````go
func ReadOrganizationMember(ctx context.Context, query Queryer, organization, actor string, lock bool) (domain.OrganizationMember, error) {
	var value domain.OrganizationMember
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := query.QueryRowContext(ctx, "SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?"+suffix, organization, actor).Scan(&value.Role, &value.State, &value.Epoch, &value.Revision)
	return value, err
}

````

## 第 40 步：Organizations.List

插入位置：第 39 步代码之后，紧接追加；完成稿约第 492 行。

本步声明：`func (r *Organizations) List(ctx context.Context, actor, last string, limit int) ([]domain.OrganizationEntry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o J …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o J …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o J …</code>（长表达式，按本步完整代码继续） |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o J …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationEntry</code> | 使用 <code>domain.OrganizationEntry</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, &quot;SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o J …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;org.ID, &amp;org.Name, &amp;org.OwnerID, &amp;org.Lifecycle, &amp;org.Revision, &amp;org.MemberRevision, &amp;value.MemberEpoch) != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;org.ID, &amp;org.Name, &amp;org.OwnerID, &amp;org.Lifecycle, &amp;org.Revision, &amp;org.MemberRevision, &amp;value.MemberEpoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return values, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=23562:24646 -->
````go
func (r *Organizations) List(ctx context.Context, actor, last string, limit int) ([]domain.OrganizationEntry, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o JOIN organization_member m ON m.organization_id=o.id AND m.user_id=? AND m.state='ACTIVE' WHERE o.lifecycle='ACTIVE' AND o.id>? ORDER BY o.id LIMIT ?", actor, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationEntry{}
	for rows.Next() {
		var value domain.OrganizationEntry
		org := &value.Organization
		if rows.Scan(&org.ID, &org.Name, &org.OwnerID, &org.Lifecycle, &org.Revision, &org.MemberRevision, &value.MemberEpoch) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

````

## 第 41 步：Organizations.Members

插入位置：第 40 步代码之后，紧接追加；完成稿约第 513 行。

本步声明：`func (r *Organizations) Members(ctx context.Context, organization, last string, limit int, search ...string) ([]domain.OrganizationMember, error)`。

**这一段为什么这样写：** 在唯一拥有用户昵称的Account库内一次JOIN成员、用户、组织和创建资格，过滤后稳定分页，同时返回输出重验所需的成员/身份代次。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次目录读取的取消上下文。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state, 		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch, 		CASE WHEN m.state&lt;&gt;&#x27;ACTIVE&#x27; THEN &#x27;NONE&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 已经通过当前成员准入的组织ID。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state, 		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch, 		CASE WHEN m.state&lt;&gt;&#x27;ACTIVE&#x27; THEN &#x27;NONE&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 签名游标解析出的上一页最后账号ID；首页为0。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state, 		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch, 		CASE WHEN m.state&lt;&gt;&#x27;ACTIVE&#x27; THEN &#x27;NONE&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>limit</code> | <code>int</code> | 仓储扫描上限，调用方传展示页大小+1以探测下一页。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state, 		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch, 		CASE WHEN m.state&lt;&gt;&#x27;ACTIVE&#x27; THEN &#x27;NONE&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>search</code> | <code>...string</code> | 可选字符串切片；仅首项是已trim并小写的q，缺省为空即不筛选。<br>在此声明中的实际使用：<code>len(search)</code>；<code>query = search&#91;0&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationMember</code> | 按原user_id升序的成员投影，尚可能包含用于more探测的最后一项。<br>实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | SQL或行扫描失败；不能把依赖故障伪装为空目录。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(search) &gt; 0</code>。
- 执行 <code>r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state, 		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch, 		CASE WHEN m.state&lt;&gt;&#x27;ACTIVE&#x27; THEN &#x27;NONE&#x27; …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;value.UserID, &amp;value.DisplayName, &amp;value.Role, &amp;value.State, &amp;value.Revision, &amp;value.Epoch, &amp;value.IdentityEpoch, &amp;value.KnowledgeBaseCreation) != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.UserID, &amp;value.DisplayName, &amp;value.Role, &amp;value.State, &amp;value.Revision, &amp;value.Epoch, &amp;value.IdentityEpoch, &amp;value.KnowledgeBaseCreation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;PAGE_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return values, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=24646:26613 -->
````go
func (r *Organizations) Members(ctx context.Context, organization, last string, limit int, search ...string) ([]domain.OrganizationMember, error) {
	query := ""
	if len(search) > 0 {
		query = search[0]
	}
	identity := strings.TrimLeft(query, "0")
	// 与Identity的建库规则一致：所有者/管理员由角色获得资格，普通成员读取独立授权。
	// 同一SQL读取，避免逐个成员查询造成状态混杂或N+1请求；分页仍绑定member_revision。
	rows, err := r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state,
		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch,
		CASE WHEN m.state<>'ACTIVE' THEN 'NONE'
		     WHEN o.owner_id=m.user_id OR m.role='ADMIN' THEN 'ROLE'
		     WHEN g.state='ACTIVE' THEN 'GRANTED' ELSE 'NONE' END
		FROM organization_member m JOIN user u ON u.id=m.user_id
		JOIN organization o ON o.id=m.organization_id
		LEFT JOIN organization_creation_grant g ON g.organization_id=m.organization_id
		  AND g.user_id=m.user_id AND g.capability='CREATE_KNOWLEDGE_BASE'
		WHERE m.organization_id=? AND m.state<>'CLOSED' AND m.user_id>?
		AND (?='' OR LOCATE(?,LOWER(u.display_name))>0 OR CAST(m.user_id AS CHAR)=?)
		ORDER BY m.user_id LIMIT ?`, organization, last, query, query, identity, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationMember{}
	for rows.Next() {
		var value domain.OrganizationMember
		if rows.Scan(&value.UserID, &value.DisplayName, &value.Role, &value.State, &value.Revision, &value.Epoch, &value.IdentityEpoch, &value.KnowledgeBaseCreation) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

// organization_transition.go：保留原实现，按业务合并文件。

````

## 第 42 步：Organizations.LockTransitionMember

插入位置：第 41 步代码之后，紧接追加；完成稿约第 553 行。

本步声明：`func (r *Organizations) LockTransitionMember(ctx context.Context, tx *sql.Tx, organization, user string) (domain.OrganizationMember, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT role, state, CAST(revision AS CHAR), epoch 		FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT role, state, CAST(revision AS CHAR), epoch 		FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT role, state, CAST(revision AS CHAR), epoch 		FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationMember</code> | 使用 <code>domain.OrganizationMember</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>member</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT role, state, CAST(revision AS CHAR), epoch 		FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code>。
- 此处通过 <code>return member, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=26613:27072 -->
````go
func (r *Organizations) LockTransitionMember(ctx context.Context, tx *sql.Tx, organization, user string) (domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := tx.QueryRowContext(ctx, `
		SELECT role, state, CAST(revision AS CHAR), epoch
		FROM organization_member
		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user).
		Scan(&member.Role, &member.State, &member.Revision, &member.Epoch)
	return member, err
}

````

## 第 43 步：Organizations.PrepareMemberTransition

插入位置：第 42 步代码之后，紧接追加；完成稿约第 563 行。

本步声明：`func (r *Organizations) PrepareMemberTransition(ctx context.Context, tx *sql.Tx, organization, user string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization_member 		  SET state=&#x27;REVOKING&#x27;, epoch=epoch+1, revision=revision+1 		  WHERE organization_id=? AND user_id=?`, &#91;&#93;any{organiza …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization_member 		  SET state=&#x27;REVOKING&#x27;, epoch=epoch+1, revision=revision+1 		  WHERE organization_id=? AND user_id=?`, &#91;&#93;any{organiza …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization_member 		  SET state=&#x27;REVOKING&#x27;, epoch=epoch+1, revision=revision+1 		  WHERE organization_id=? AND user_id=?`, &#91;&#93;any{organiza …</code>（长表达式，按本步完整代码继续） |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization_member 		  SET state=&#x27;REVOKING&#x27;, epoch=epoch+1, revision=revision+1 		  WHERE organization_id=? AND user_id=?`, &#91;&#93;any{organiza …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization_member 		  SET state=&#x27;REVOKING&#x27;, epoch=epoch+1, revision=revision+1 		  WHERE organization_id=? AND user_id=?`, &#91;&#93;any{organiza …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization_member 		  SET state=&#x27;REVOKING&#x27;, epoch=epoch+1, revision=revision+1 		  WHERE organization_id=? AND user_id=?`, &#91;&#93;any{o …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 43 bytes=27072:27494 -->
````go
func (r *Organizations) PrepareMemberTransition(ctx context.Context, tx *sql.Tx, organization, user string) error {
	return execute(ctx, tx, []statement{
		{`UPDATE organization_member
		  SET state='REVOKING', epoch=epoch+1, revision=revision+1
		  WHERE organization_id=? AND user_id=?`, []any{organization, user}},
		{"UPDATE organization SET member_revision=member_revision+1 WHERE id=?", []any{organization}},
	})
}

````

## 第 44 步：Organizations.InsertMemberTransition

插入位置：第 43 步代码之后，紧接追加；完成稿约第 572 行。

本步声明：`func (r *Organizations) InsertMemberTransition(ctx context.Context, tx *sql.Tx, id, organization, actor, user, kind, expected string, plan []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续） |
| <code>plan</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,?,?,&#x27;F …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=27494:27929 -->
````go
func (r *Organizations) InsertMemberTransition(ctx context.Context, tx *sql.Tx, id, organization, actor, user, kind, expected string, plan []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO organization_transition
		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
		VALUES(?,?,?,?,?,?,'FENCING',?)`, id, organization, actor, user, kind, expected, string(plan))
	return err
}

````

## 第 45 步：Organizations.LockUserState

插入位置：第 44 步代码之后，紧接追加；完成稿约第 580 行。

本步声明：`func (r *Organizations) LockUserState(ctx context.Context, tx *sql.Tx, user string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 45 bytes=27929:28170 -->
````go
func (r *Organizations) LockUserState(ctx context.Context, tx *sql.Tx, user string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", user).Scan(&state)
	return state, err
}

````

## 第 46 步：Organizations.LockOwnerMember

插入位置：第 45 步代码之后，紧接追加；完成稿约第 586 行。

本步声明：`func (r *Organizations) LockOwnerMember(ctx context.Context, tx *sql.Tx, organization, owner string) (domain.OrganizationMember, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT epoch, role FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, owner)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT epoch, role FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, owner)</code> |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT epoch, role FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, owner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationMember</code> | 使用 <code>domain.OrganizationMember</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>member</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT epoch, role FROM organization_member 		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, owner)</code>。
- 此处通过 <code>return member, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 46 bytes=28170:28560 -->
````go
func (r *Organizations) LockOwnerMember(ctx context.Context, tx *sql.Tx, organization, owner string) (domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := tx.QueryRowContext(ctx, `
		SELECT epoch, role FROM organization_member
		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, owner).
		Scan(&member.Epoch, &member.Role)
	return member, err
}

````

## 第 47 步：Organizations.PrepareOwnerTransfer

插入位置：第 46 步代码之后，紧接追加；完成稿约第 595 行。

本步声明：`func (r *Organizations) PrepareOwnerTransfer(ctx context.Context, tx *sql.Tx, organization, owner, target string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{`UPDATE …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{`UPDATE …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{`UPDATE …</code>（长表达式，按本步完整代码继续） |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{`UPDATE …</code>（长表达式，按本步完整代码继续） |
| <code>target</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{`UPDATE …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{`UPDATE …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 		  WHERE id=?`, &#91;&#93;any{target, organization}}, 		{ …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 47 bytes=28560:29026 -->
````go
func (r *Organizations) PrepareOwnerTransfer(ctx context.Context, tx *sql.Tx, organization, owner, target string) error {
	return execute(ctx, tx, []statement{
		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1
		  WHERE id=?`, []any{target, organization}},
		{`UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1
		  WHERE organization_id=? AND user_id=?`, []any{organization, owner}},
	})
}

````

## 第 48 步：Organizations.InsertOwnerTransfer

插入位置：第 47 步代码之后，紧接追加；完成稿约第 604 行。

本步声明：`func (r *Organizations) InsertOwnerTransfer(ctx context.Context, tx *sql.Tx, id, organization, owner, target, expected string, plan []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续） |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续） |
| <code>target</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续） |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续） |
| <code>plan</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO organization_transition 		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan) 		VALUES(?,?,?,?,&#x27;TRANS …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 48 bytes=29026:29536 -->
````go
func (r *Organizations) InsertOwnerTransfer(ctx context.Context, tx *sql.Tx, id, organization, owner, target, expected string, plan []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO organization_transition
		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
		VALUES(?,?,?,?,'TRANSFER_OWNER',?,'FENCING',?)`, id, organization, owner, target, expected, string(plan))
	return err
}

// organization_write.go：保留原实现，按业务合并文件。

````

## 第 49 步：Organizations.Insert

插入位置：第 48 步代码之后，紧接追加；完成稿约第 614 行。

本步声明：`func (r *Organizations) Insert(ctx context.Context, tx *sql.Tx, name, owner string) (int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, name, owner)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, owner)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, name, owner)</code> |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, name, owner)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, owner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,&#x27;ACTIVE&#x27;)&quot;, name, owner)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,&#x27;ADMIN&#x27;,&#x27;ACTIVE&#x27;)&quot;, id, owner)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return id, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 49 bytes=29536:29988 -->
````go
func (r *Organizations) Insert(ctx context.Context, tx *sql.Tx, name, owner string) (int64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,'ACTIVE')", name, owner)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	_, err = tx.ExecContext(ctx, "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", id, owner)
	return id, err
}

````

## 第 50 步：Organizations.Rename

插入位置：第 49 步代码之后，紧接追加；完成稿约第 624 行。

本步声明：`func (r *Organizations) Rename(ctx context.Context, tx *sql.Tx, id, name string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization SET name=?,revision=revision+1 WHERE id=?&quot;, name, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization SET name=?,revision=revision+1 WHERE id=?&quot;, name, id)</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization SET name=?,revision=revision+1 WHERE id=?&quot;, name, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization SET name=?,revision=revision+1 WHERE id=?&quot;, name, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 50 bytes=29988:30198 -->
````go
func (r *Organizations) Rename(ctx context.Context, tx *sql.Tx, id, name string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization SET name=?,revision=revision+1 WHERE id=?", name, id)
	return err
}

````

## 第 51 步：Organizations.LockMemberState

插入位置：第 50 步代码之后，紧接追加；完成稿约第 629 行。

本步声明：`func (r *Organizations) LockMemberState(ctx context.Context, tx *sql.Tx, id, user string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, id, user).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, id, user)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 51 bytes=30198:30491 -->
````go
func (r *Organizations) LockMemberState(ctx context.Context, tx *sql.Tx, id, user string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", id, user).Scan(&state)
	return state, err
}

````

## 第 52 步：Organizations.LockCreationGrant

插入位置：第 51 步代码之后，紧接追加；完成稿约第 635 行。

本步声明：`func (r *Organizations) LockCreationGrant(ctx context.Context, tx *sql.Tx, id, user string) (uint64, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, id, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, id, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, id, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, id, user).Scan(&amp;revision, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, id, user)</code>。
- 此处通过 <code>return revision, state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 52 bytes=30491:30892 -->
````go
func (r *Organizations) LockCreationGrant(ctx context.Context, tx *sql.Tx, id, user string) (uint64, string, error) {
	var revision uint64
	var state string
	err := tx.QueryRowContext(ctx, "SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability='CREATE_KNOWLEDGE_BASE' FOR UPDATE", id, user).Scan(&revision, &state)
	return revision, state, err
}

````

## 第 53 步：Organizations.WriteCreationGrant

插入位置：第 52 步代码之后，紧接追加；完成稿约第 642 行。

本步声明：`func (r *Organizations) WriteCreationGrant(ctx context.Context, tx *sql.Tx, id, user, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,&#x27;CREATE_KNOWLEDGE_BASE&#x27;,?) ON DUPLICATE KEY UPDATE state=VALUES(st …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE organization SET member_revision=member_revision+1 WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,&#x27;CREATE_KNOWLEDGE_BASE&#x27;,?) ON DUPLICATE KEY UPDATE state=VALUES(st …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE organization SET member_revision=member_revision+1 WHERE id=?&quot;, id)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,&#x27;CREATE_KNOWLEDGE_BASE&#x27;,?) ON DUPLICATE KEY UPDATE state=VALUES(st …</code>（长表达式，按本步完整代码继续） |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,&#x27;CREATE_KNOWLEDGE_BASE&#x27;,?) ON DUPLICATE KEY UPDATE state=VALUES(st …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,&#x27;CREATE_KNOWLEDGE_BASE&#x27;,?) ON DUPLICATE KEY UPDATE state=VALUES(st …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization SET member_revision=member_revision+1 WHERE id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 53 bytes=30892:31388 -->
````go
func (r *Organizations) WriteCreationGrant(ctx context.Context, tx *sql.Tx, id, user, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,'CREATE_KNOWLEDGE_BASE',?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", id, user, state)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE organization SET member_revision=member_revision+1 WHERE id=?", id)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/organization_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/03-organizations.md)。
