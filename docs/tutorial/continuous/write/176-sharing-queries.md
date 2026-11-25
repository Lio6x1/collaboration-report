# 分步编写 `services/document/internal/repository/document/sharing_queries.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/176-sharing-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存邀请、申请、申请审批结果和只属于发起人的操作回执。

邀请的授权清单要在申请时固定，审批与grant写入共同提交，不能审批时偷偷换成新的邀请范围。

写之前先按这条执行路线理解各段的连接：

1. InvitationByHash定位邀请
2. InsertApplication记录授权快照
3. ApproveInvitationGrant写grant
4. DecideApplication落审批状态
5. 列表先按状态与管理归档过滤，再按ID降序取limit+1；管理归档不影响本人申请历史。

本文件需要守住的边界：

- InvitationFact只是事实，是否过期仍须在事务内判断
- ActorOperationResult必须按actor限制查询。
- 查询只有sql.ErrNoRows转为EntityNotFound；数据库连接或扫描失败不能伪装成记录不存在。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/sharing_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/sharing_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/sharing_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/sharing_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:350 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：ApplicationRow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func ApplicationRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.DocumentApplication, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(invitation_id AS CHAR),CAST(applicant_id AS CHAR),invitation_validated_at,requested_grants,state,reas …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(invitation_id AS CHAR),CAST(applicant_id AS CHAR),invitation_validated_at,requested_grants,state,reas …</code>（长表达式，按本步完整代码继续） |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.DocumentApplication</code> | 实际返回表达式：<code>v</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;APPLICATION_NOT_FOUND&quot;)</code>；<code>err</code>；<code>errors.New(&quot;APPLICATION_GRANTS_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(invitation_id AS CHAR),CAST(applicant_id AS CHAR),invitation_validated_at,requested_grants,state,reas …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(invitation_id AS CHAR),CAST(applicant_id AS CHAR),invitation_validated_at,requested_grants,state,reas …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return v, domain.Violation(domain.EntityNotFound, &quot;APPLICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return v, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;v.RequestedGrants) != nil</code>。条件成立时执行 <code>return v, errors.New(&quot;APPLICATION_GRANTS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return v, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=350:1411 -->
````go
func ApplicationRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.DocumentApplication, error) {
	var v dto.DocumentApplication
	var raw []byte
	var validated, created time.Time
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(invitation_id AS CHAR),CAST(applicant_id AS CHAR),invitation_validated_at,requested_grants,state,reason,CAST(revision AS CHAR),created_at FROM document_application WHERE id=?"+suffix, id).Scan(&v.Id, &v.DocumentId, &v.InvitationId, &v.ApplicantId, &validated, &raw, &v.State, &v.Reason, &v.Revision, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return v, domain.Violation(domain.EntityNotFound, "APPLICATION_NOT_FOUND")
	}
	if err != nil {
		return v, err
	}
	if json.Unmarshal(raw, &v.RequestedGrants) != nil {
		return v, errors.New("APPLICATION_GRANTS_UNAVAILABLE")
	}
	v.InvitationValidatedAt = validated.UTC().Format(time.RFC3339Nano)
	stamp := created.UTC().Format(time.RFC3339Nano)
	v.CreatedAt = &stamp
	return v, nil
}
````

## 第 3 步：PendingApplication

插入位置：第 2 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func PendingApplication(ctx context.Context, tx *sql.Tx, document, actor uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; ORDER BY id LIMIT 1 FOR UPDATE&quot;, document, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; ORDER BY id LIMIT 1 FOR UPDATE&quot;, document, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; ORDER BY id LIMIT 1 FOR UPDATE&quot;, document, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; ORDER BY id LIMIT 1 FOR UPDATE&quot;, document, actor).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state=&#x27;PENDING&#x27; ORDER BY id LIMIT 1 FOR UPDATE&quot;, document, actor)</code>。
- 此处通过 <code>return id, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1411:1731 -->
````go
func PendingApplication(ctx context.Context, tx *sql.Tx, document, actor uint64) (uint64, error) {
	var id uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state='PENDING' ORDER BY id LIMIT 1 FOR UPDATE", document, actor).Scan(&id)
	return id, err
}
````

## 第 4 步：InsertApplication

插入位置：第 3 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func InsertApplication(ctx context.Context, tx *sql.Tx, document, invitation, actor uint64, grants, reason string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |
| <code>invitation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |
| <code>grants</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES( …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state)  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1731:2144 -->
````go
func InsertApplication(ctx context.Context, tx *sql.Tx, document, invitation, actor uint64, grants, reason string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES(?,?,?,?,TRUE,UTC_TIMESTAMP(6),?,'PENDING')", document, invitation, actor, grants, reason)
}
````

## 第 5 步：ApproveInvitationGrant

插入位置：第 4 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func ApproveInvitationGrant(ctx context.Context, tx *sql.Tx, document uint64, applicant string, grant dto.InvitationGrant) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,&#x27;ACTIVE&#x27;) ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,revision= …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,&#x27;ACTIVE&#x27;) ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,revision= …</code>（长表达式，按本步完整代码继续） |
| <code>applicant</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,&#x27;ACTIVE&#x27;) ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,revision= …</code>（长表达式，按本步完整代码继续） |
| <code>grant</code> | <code>dto.InvitationGrant</code> | 在此声明中的实际使用：<code>grant.ScopeKind</code>；<code>grant.ScopeId</code>；<code>grant.Capability</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,&#x27;ACTIVE&#x27;) ON DUPLICATE KEY UPDATE state=&#x27;ACTIVE&#x27;,revision= …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2144:2564 -->
````go
func ApproveInvitationGrant(ctx context.Context, tx *sql.Tx, document uint64, applicant string, grant dto.InvitationGrant) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,'ACTIVE') ON DUPLICATE KEY UPDATE state='ACTIVE',revision=revision+1", document, applicant, grant.ScopeKind, grant.ScopeId, grant.Capability)
	return err
}
````

## 第 6 步：DecideApplication

插入位置：第 5 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func DecideApplication(ctx context.Context, tx *sql.Tx, id, reviewer uint64, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, reviewer, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, reviewer, id)</code> |
| <code>reviewer</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, reviewer, id)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, reviewer, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?&quot;, state, reviewer, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2564:2817 -->
````go
func DecideApplication(ctx context.Context, tx *sql.Tx, id, reviewer uint64, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?", state, reviewer, id)
	return err
}
````

## 第 7 步：ApplicationIDs

插入位置：第 6 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func ApplicationIDs(ctx context.Context, q Queryer, key uint64, byDocument bool, last string, budget int, filter string) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM document_application WHERE &quot;+condition+&quot; AND (?=&#x27;0&#x27; OR id&lt;?) ORDER BY id DESC LIMIT ?&quot;, key, last, last, budget)</code> |
| <code>q</code> | <code>Queryer</code> | 调用者传入的数据库或同一事务；查询候选不另起连接。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>key</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM document_application WHERE &quot;+condition+&quot; AND (?=&#x27;0&#x27; OR id&lt;?) ORDER BY id DESC LIMIT ?&quot;, key, last, last, budget)</code> |
| <code>byDocument</code> | <code>bool</code> | true按文档筛申请，key是文档编号；false按申请人筛，key是账号编号。 |
| <code>last</code> | <code>string</code> | 上一页最后ID，0表示首页；后页只取更小的ID。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM document_application WHERE &quot;+condition+&quot; AND (?=&#x27;0&#x27; OR id&lt;?) ORDER BY id DESC LIMIT ?&quot;, key, last, last, budget)</code> |
| <code>budget</code> | <code>int</code> | 读取上限，业务传每页条数加1来判断还有后页。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM document_application WHERE &quot;+condition+&quot; AND (?=&#x27;0&#x27; OR id&lt;?) ORDER BY id DESC LIMIT ?&quot;, key, last, last, budget)</code> |
| <code>filter</code> | <code>string</code> | PENDING只取待处理；PROCESSED只取已处理；ALL不过滤状态。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;SHARING_FILTER_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;APPLICATIONS_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>byDocument</code>。
- 此处通过 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;SHARING_FILTER_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryContext(ctx, &quot;SELECT id FROM document_application WHERE &quot;+condition+&quot; AND (?=&#x27;0&#x27; OR id&lt;?) ORDER BY id DESC LIMIT ?&quot;, key, last, last, budget)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;id) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;APPLICATIONS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2817:3895 -->
````go
func ApplicationIDs(ctx context.Context, q Queryer, key uint64, byDocument bool, last string, budget int, filter string) ([]uint64, error) {
	condition := "applicant_id=?"
	if byDocument {
		condition = "document_id=?"
		// 管理者归档不抹去申请人的个人进度。
		condition += " AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record_kind='APPLICATION' AND a.record_id=document_application.id)"
	}
	switch filter {
	case "PENDING":
		condition += " AND state='PENDING'"
	case "PROCESSED":
		condition += " AND state<>'PENDING'"
	case "ALL":
	default:
		return nil, domain.Violation(domain.InvalidRequest, "SHARING_FILTER_INVALID")
	}
	rows, err := q.QueryContext(ctx, "SELECT id FROM document_application WHERE "+condition+" AND (?='0' OR id<?) ORDER BY id DESC LIMIT ?", key, last, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, errors.New("APPLICATIONS_UNAVAILABLE")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
````

## 第 8 步：ActorOperationResult

插入位置：第 7 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func ActorOperationResult(ctx context.Context, tx *sql.Tx, operation string, actor uint64) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?&quot;, operation, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?&quot;, operation, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?&quot;, operation, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?&quot;, operation, actor).Scan(&amp;raw) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?&quot;, operation, actor).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?&quot;, operation, actor)</code>。
- 此处通过 <code>return raw, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3895:4330 -->
````go
func ActorOperationResult(ctx context.Context, tx *sql.Tx, operation string, actor uint64) ([]byte, error) {
	var raw []byte
	if tx.QueryRowContext(ctx, "SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?", operation, actor).Scan(&raw) != nil {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_NOT_FOUND")
	}
	return raw, nil
}

// invitation.go：保留原实现，按业务合并文件。

````

## 第 9 步：InvitationFact

插入位置：第 8 步代码之后，紧接追加；完成稿约第 96 行。

InvitationFact保留真实期限与密文；是否到期由用例决定，读取不更新持久状态。

本步声明：`type InvitationFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Invitation</code> | <code>dto.DocumentInvitation</code> | 对外可读的邀请信息；不包含可直接使用的原文令牌。 |
| <code>ExpiresAt</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |
| <code>Ciphertext</code> | <code>&#91;&#93;byte</code> | 加密保存的邀请令牌，解密时还须绑定原文档作为附加认证数据。 |

<!-- write-step: 9 bytes=4330:4546 -->
````go
// InvitationFact保留真实期限与密文；是否到期由用例决定，读取不更新持久状态。
type InvitationFact struct {
	Invitation dto.DocumentInvitation
	ExpiresAt  time.Time
	Ciphertext []byte
}

````

## 第 10 步：InvitationRow

插入位置：第 9 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func InvitationRow(ctx context.Context, q Queryer, id uint64, lock bool) (InvitationFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),state,expires_at,requested_grants,CAST(revision AS CHAR),token_ciphertext,created_at FROM document_invitation WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),state,expires_at,requested_grants,CAST(revision AS CHAR),token_ciphertext,created_at FROM document_invitation WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>InvitationFact</code> | 使用 <code>InvitationFact</code> 类型；其定义处的职责：InvitationFact保留真实期限与密文；是否到期由用例决定，读取不更新持久状态。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;INVITATION_NOT_FOUND&quot;)</code>；<code>err</code>；<code>errors.New(&quot;INVITATION_GRANTS_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),state,expires_at,requested_grants,CAST(revision AS CHAR),token_ciphertext,created_at FROM document_invitat …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),state,expires_at,requested_grants,CAST(revision AS CHAR),token_ciphertext,created_at FROM document_invitation WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return fact, domain.Violation(domain.EntityNotFound, &quot;INVITATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fact, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;v.RequestedGrants) != nil</code>。条件成立时执行 <code>return fact, errors.New(&quot;INVITATION_GRANTS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fact, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4546:5465 -->
````go
func InvitationRow(ctx context.Context, q Queryer, id uint64, lock bool) (InvitationFact, error) {
	var fact InvitationFact
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	v := &fact.Invitation
	var created time.Time
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),state,expires_at,requested_grants,CAST(revision AS CHAR),token_ciphertext,created_at FROM document_invitation WHERE id=?"+suffix, id).Scan(&v.Id, &v.DocumentId, &v.State, &fact.ExpiresAt, &raw, &v.Revision, &fact.Ciphertext, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return fact, domain.Violation(domain.EntityNotFound, "INVITATION_NOT_FOUND")
	}
	if err != nil {
		return fact, err
	}
	if json.Unmarshal(raw, &v.RequestedGrants) != nil {
		return fact, errors.New("INVITATION_GRANTS_UNAVAILABLE")
	}
	stamp := created.UTC().Format(time.RFC3339Nano)
	v.CreatedAt = &stamp
	return fact, nil
}
````

## 第 11 步：InvitationByHash

插入位置：第 10 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`func InvitationByHash(ctx context.Context, q Queryer, hash []byte) (uint64, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,document_id FROM document_invitation WHERE token_hash=?&quot;, hash)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,document_id FROM document_invitation WHERE token_hash=?&quot;, hash)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>invitation</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>document</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;INVITATION_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id FROM document_invitation WHERE token_hash=?&quot;, hash).Scan(&amp;invitation, &amp;document) != nil</code>。条件成立时执行 <code>return 0, 0, domain.Violation(domain.EntityNotFound, &quot;INVITATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id FROM document_invitation WHERE token_hash=?&quot;, hash).Scan(&amp;invitation, &amp;document)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id FROM document_invitation WHERE token_hash=?&quot;, hash)</code>。
- 此处通过 <code>return invitation, document, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5465:5849 -->
````go
func InvitationByHash(ctx context.Context, q Queryer, hash []byte) (uint64, uint64, error) {
	var invitation, document uint64
	if q.QueryRowContext(ctx, "SELECT id,document_id FROM document_invitation WHERE token_hash=?", hash).Scan(&invitation, &document) != nil {
		return 0, 0, domain.Violation(domain.EntityNotFound, "INVITATION_NOT_FOUND")
	}
	return invitation, document, nil
}
````

## 第 12 步：InsertInvitation

插入位置：第 11 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`func InsertInvitation(ctx context.Context, tx *sql.Tx, document, actor uint64, hash, encrypted []byte, grants string, expires time.Time) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>encrypted</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>grants</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-document-i …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,&#x27;dev-doc …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5849:6273 -->
````go
func InsertInvitation(ctx context.Context, tx *sql.Tx, document, actor uint64, hash, encrypted []byte, grants string, expires time.Time) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,'dev-document-invitation-v1',?,'OPEN',?)", document, actor, hash, encrypted, grants, expires)
}
````

## 第 13 步：CloseInvitation

插入位置：第 12 步代码之后，紧接追加；完成稿约第 136 行。

本步声明：`func CloseInvitation(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;OPEN&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;OPEN&#x27;&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;OPEN&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6273:6492 -->
````go
func CloseInvitation(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_invitation SET state='CLOSED',revision=revision+1 WHERE id=? AND state='OPEN'", id)
	return err
}
````

## 第 14 步：InvitationIDs

插入位置：第 13 步代码之后，紧接追加；完成稿约第 140 行。

本步声明：`func InvitationIDs(ctx context.Context, tx *sql.Tx, document uint64, last string, budget int, filter string) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM document_invitation WHERE document_id=? AND (?=&#x27;0&#x27; OR id&lt;?)&quot;+condition+&quot; AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM document_invitation WHERE document_id=? AND (?=&#x27;0&#x27; OR id&lt;?)&quot;+condition+&quot; AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 降序游标中的最后ID，0表示首页。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM document_invitation WHERE document_id=? AND (?=&#x27;0&#x27; OR id&lt;?)&quot;+condition+&quot; AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record …</code>（长表达式，按本步完整代码继续） |
| <code>budget</code> | <code>int</code> | 页大小加1，先过滤再截取，避免失效记录挤占有效页。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM document_invitation WHERE document_id=? AND (?=&#x27;0&#x27; OR id&lt;?)&quot;+condition+&quot; AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record …</code>（长表达式，按本步完整代码继续） |
| <code>filter</code> | <code>string</code> | ACTIVE为OPEN且未过期；INACTIVE为关闭或已过期；ALL包含两者，均排除归档。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;SHARING_FILTER_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;INVITATIONS_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;SHARING_FILTER_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id FROM document_invitation WHERE document_id=? AND (?=&#x27;0&#x27; OR id&lt;?)&quot;+condition+&quot; AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;id) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;INVITATIONS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6492:7491 -->
````go
func InvitationIDs(ctx context.Context, tx *sql.Tx, document uint64, last string, budget int, filter string) ([]uint64, error) {
	condition := ""
	switch filter {
	case "ACTIVE":
		condition = " AND state='OPEN' AND expires_at>UTC_TIMESTAMP(6)"
	case "INACTIVE":
		condition = " AND (state<>'OPEN' OR expires_at<=UTC_TIMESTAMP(6))"
	case "ALL":
	default:
		return nil, domain.Violation(domain.InvalidRequest, "SHARING_FILTER_INVALID")
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM document_invitation WHERE document_id=? AND (?='0' OR id<?)"+condition+" AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record_kind='INVITATION' AND a.record_id=document_invitation.id) ORDER BY id DESC LIMIT ?", document, last, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, errors.New("INVITATIONS_UNAVAILABLE")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

````

## 第 15 步：ArchiveSharingRecord

插入位置：第 14 步代码之后，紧接追加；完成稿约第 167 行。

本步声明：`func ArchiveSharingRecord(ctx context.Context, tx *sql.Tx, kind string, id, document, actor uint64) error`。

**这一段为什么这样写：** 同一事务追加归档事实并推进原记录修订；不触碰content_grant或原审批状态。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)&quot;, kind, id, document, actor)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET revision=revision+1 WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>kind</code> | <code>string</code> | 服务端固定类别INVITATION或APPLICATION，映射到可信表名；不能直接拼客户端表名。<br>在此声明中的实际使用：<code>kind == &quot;APPLICATION&quot;</code>；<code>kind != &quot;INVITATION&quot;</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)&quot;, kind, id, document, actor)</code> |
| <code>id</code> | <code>uint64</code> | 已锁定且核对修订/状态的原记录编号。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)&quot;, kind, id, document, actor)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET revision=revision+1 WHERE id=?&quot;, id)</code> |
| <code>document</code> | <code>uint64</code> | 已重验管理资格的所属文档。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)&quot;, kind, id, document, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 发起归档的真实账号，保存操作责任人。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)&quot;, kind, id, document, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;SHARING_RECORD_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>kind == &quot;APPLICATION&quot;</code>。
- 检查 <code>kind != &quot;INVITATION&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;SHARING_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)&quot;, kind, id, document, actor)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET revision=revision+1 WHERE id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 15 bytes=7491:8212 -->
````go
func ArchiveSharingRecord(ctx context.Context, tx *sql.Tx, kind string, id, document, actor uint64) error {
	table := "document_invitation"
	if kind == "APPLICATION" {
		table = "document_application"
	} else if kind != "INVITATION" {
		return domain.Violation(domain.InvalidRequest, "SHARING_RECORD_INVALID")
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)", kind, id, document, actor); err != nil {
		return err
	}
	// 与归档事实在同一事务推进修订，挡住并发旧输入；原业务状态和授权保持原样。
	_, err := tx.ExecContext(ctx, "UPDATE "+table+" SET revision=revision+1 WHERE id=?", id)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/sharing_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
